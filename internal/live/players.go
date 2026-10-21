package live

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"f1-scoreboard/internal/recording"
	"f1-scoreboard/internal/telemetry"
)

// ErrSessionRecording indica un'operazione non possibile su una sessione ancora in corso.
var ErrSessionRecording = errors.New("la sessione è ancora in registrazione")

// InvalidError è una richiesta non valida, con il messaggio da mostrare all'utente.
type InvalidError struct{ msg string }

func (e *InvalidError) Error() string { return e.msg }

func invalid(format string, args ...any) error {
	return &InvalidError{msg: fmt.Sprintf(format, args...)}
}

// dbtx è ciò che hanno in comune *sql.DB e *sql.Tx: le funzioni che seguono
// funzionano sia dentro sia fuori da una transazione.
type dbtx interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// sessionData sono i pacchetti che servono per sapere chi ha giocato una sessione e come è finita.
type sessionData struct {
	header         telemetry.Header   // l'ultimo: contiene gli indici delle auto dei giocatori
	session        *telemetry.Session // serve per riconoscere le sprint (struttura del weekend)
	participants   *telemetry.Participants
	classification *telemetry.FinalClassification
	// assigned sono i piloti scelti dalla pagina Live (indice dell'auto → pilota,
	// nil se nessuno): per queste auto non si tira a indovinare.
	assigned map[int]*int64
}

// controllerOf restituisce il controller (1 o 2) che guida l'auto i, 0 se nessuno.
func controllerOf(h telemetry.Header, i int) int {
	switch i {
	case int(h.PlayerCarIndex):
		return 1
	case int(h.SecondaryPlayerCarIndex):
		return 2
	}
	return 0
}

// analyzeFile legge da una registrazione i pacchetti della sessione uid.
// Un file troncato (server spento di colpo) viene letto fin dove è valido.
func analyzeFile(path string, uid uint64) (sessionData, error) {
	var d sessionData
	f, err := os.Open(path)
	if err != nil {
		return d, err
	}
	defer f.Close()
	r, err := recording.NewReader(f)
	if err != nil {
		return d, err
	}
	for {
		p, err := r.Next()
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return d, nil
		}
		if err != nil {
			return d, err
		}
		h, err := telemetry.ParseHeader(p.Data)
		if err != nil || h.SessionUID != uid {
			continue
		}
		// Si decodificano solo i pacchetti utili: sono pochi rispetto al totale.
		switch h.PacketID {
		case telemetry.PacketParticipants, telemetry.PacketFinalClassification, telemetry.PacketSession:
		default:
			continue
		}
		packet, err := telemetry.Parse(p.Data)
		if err != nil {
			continue
		}
		d.header = h
		switch packet := packet.(type) {
		case *telemetry.Session:
			d.session = packet
		case *telemetry.Participants:
			d.participants = packet
		case *telemetry.FinalClassification:
			d.classification = packet
		}
	}
}

// analyzePending estrae i giocatori dalle sessioni registrate prima che il
// server lo facesse da solo (o interrotte da uno spegnimento improvviso).
func (s *Service) analyzePending() {
	type pending struct {
		id   int64
		uid  string
		file string
	}
	rows, err := s.db.Query("SELECT id, session_uid, file FROM telemetry_sessions WHERE analyzed = 0 AND ended_at IS NOT NULL")
	if err != nil {
		log.Printf("telemetria: %v", err)
		return
	}
	var list []pending
	for rows.Next() {
		var p pending
		if rows.Scan(&p.id, &p.uid, &p.file) == nil {
			list = append(list, p)
		}
	}
	rows.Close()

	for _, p := range list {
		var d sessionData
		uid, err := strconv.ParseUint(p.uid, 16, 64)
		if err == nil {
			d, err = analyzeFile(filepath.Join(s.opts.Dir, p.file), uid)
		}
		// Senza file (registrazione disattivata o cancellata a mano) non c'è nulla da estrarre:
		// la sessione viene comunque segnata come analizzata per non riprovare a ogni avvio.
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("telemetria: lettura della sessione %d: %v", p.id, err)
		}
		if err := s.savePlayers(p.id, d); err != nil {
			log.Printf("telemetria: salvataggio dei giocatori della sessione %d: %v", p.id, err)
		}
	}
	if len(list) > 0 {
		log.Printf("telemetria: estratti i giocatori di %d sessioni registrate", len(list))
	}
	// Dopo l'analisi, così le gare create hanno subito i risultati.
	s.adoptPending()
	s.recoverPending()
	s.backfillStats()
	s.DataChanged()
}

// recoverPending salva nel torneo in corso (o in "Nessun torneo") le sessioni
// finite con la classifica finale che non sono state salvate: il server si è
// spento di colpo prima di farlo, oppure il salvataggio non è riuscito. Le
// sessioni in corso non hanno ancora ended_at e restano fuori.
func (s *Service) recoverPending() {
	type pending struct {
		id          int64
		uid, file   string
		sessionType *int
	}
	rows, err := s.db.Query("SELECT id, session_uid, file, session_type FROM telemetry_sessions WHERE pending_save = 1 AND ended_at IS NOT NULL")
	if err != nil {
		log.Printf("telemetria: %v", err)
		return
	}
	var list []pending
	for rows.Next() {
		var p pending
		if rows.Scan(&p.id, &p.uid, &p.file, &p.sessionType) == nil {
			list = append(list, p)
		}
	}
	rows.Close()

	for _, p := range list {
		raceType := ""
		if p.sessionType != nil {
			raceType = raceTypeFor(telemetry.SessionType(*p.sessionType))
		}
		// Le sprint si riconoscono dalla struttura del weekend, che è solo nella registrazione.
		if uid, err := strconv.ParseUint(p.uid, 16, 64); err == nil {
			if d, err := analyzeFile(filepath.Join(s.opts.Dir, p.file), uid); err == nil && d.session != nil {
				raceType = raceTypeOf(d.session)
			}
		}
		t, _, err := s.addRaceToTournament(p.id, raceType)
		if err != nil {
			log.Printf("telemetria: salvataggio della sessione %d nel torneo: %v", p.id, err)
			continue
		}
		if t != nil {
			log.Printf("telemetria: sessione %d rimasta in sospeso salvata nel torneo %q", p.id, t.Name)
		}
	}
	if len(list) > 0 {
		s.DataChanged()
	}
}

// adoptPending mette in "Nessun torneo" le sessioni finite registrate prima
// che ogni sessione finita avesse un torneo (segnate dalla migrazione 006).
// Succede una volta sola: una gara eliminata a mano dopo non ricompare.
func (s *Service) adoptPending() {
	rows, err := s.db.Query("SELECT id FROM telemetry_sessions WHERE needs_tournament = 1 ORDER BY id")
	if err != nil {
		log.Printf("telemetria: %v", err)
		return
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	if len(ids) == 0 {
		return
	}

	var free int64
	if err := s.db.QueryRow("SELECT id FROM tournaments WHERE free = 1").Scan(&free); err != nil {
		log.Printf("telemetria: torneo \"Nessun torneo\" non trovato: %v", err)
		return
	}
	adopted := 0
	for _, id := range ids {
		// Nel frattempo la sessione potrebbe essere stata assegnata a mano.
		var raceID *int64
		if err := s.db.QueryRow("SELECT race_id FROM telemetry_sessions WHERE id = ?", id).Scan(&raceID); err != nil {
			log.Printf("telemetria: sessione %d: %v", id, err)
			continue
		}
		if raceID == nil {
			if _, err := s.LinkNewRace(id, free, ""); err != nil {
				log.Printf("telemetria: sessione %d in \"Nessun torneo\": %v", id, err)
				continue
			}
			adopted++
		}
		s.exec("UPDATE telemetry_sessions SET needs_tournament = 0 WHERE id = ?", id)
	}
	if adopted > 0 {
		log.Printf("telemetria: %d sessioni finite senza torneo spostate in \"Nessun torneo\"", adopted)
	}
}

// savePlayers salva i giocatori umani della sessione, associandoli quando
// possibile a un pilota del segnapunti, e aggiorna i risultati della gara collegata.
func (s *Service) savePlayers(sessionID int64, d sessionData) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // non fa nulla se la transazione è già stata confermata

	if _, err := tx.Exec("DELETE FROM telemetry_players WHERE session_id = ?", sessionID); err != nil {
		return err
	}
	var cars *int
	if p := d.participants; p != nil {
		n := p.NumCars()
		cars = &n
		taken := map[int64]bool{} // un pilota non può guidare due auto nella stessa gara
		for _, id := range d.assigned {
			if id != nil {
				taken[*id] = true
			}
		}
		for i, car := range p.Cars {
			if i >= n {
				break
			}
			// Il gioco segna come umani tutti i giocatori, anche quelli online;
			// gli indici dell'header coprono anche lo split-screen.
			human := !car.AIControlled || i == int(d.header.PlayerCarIndex) || i == int(d.header.SecondaryPlayerCarIndex)
			if !human {
				continue
			}
			var position, status *int
			if c := d.classification; c != nil && i < len(c.Cars) {
				pos, st := int(c.Cars[i].Position), int(c.Cars[i].ResultStatus)
				if pos > 0 {
					position = &pos
				}
				status = &st
			}
			c := controllerOf(d.header, i)
			var controller *int
			if c > 0 {
				controller = &c
			}
			playerID, ok := d.assigned[i]
			if !ok {
				if playerID, err = guessPlayer(tx, sessionID, c, car.Name, taken); err != nil {
					return err
				}
			}
			st := statsOf(d.classification, i)
			_, err = tx.Exec(`
				INSERT INTO telemetry_players (session_id, car_index, name, team, race_number, position, result_status, player_id, controller,
					best_lap_ms, grid_position, laps, pit_stops, penalties_s, race_time_ms)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				sessionID, i, car.Name, car.TeamID.String(), int(car.RaceNumber), position, status, playerID, controller,
				st.bestLap, st.grid, st.laps, st.pitStops, st.penalties, st.raceTime)
			if err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec("UPDATE telemetry_sessions SET analyzed = 1, cars = ? WHERE id = ?", cars, sessionID); err != nil {
		return err
	}
	if c := d.classification; c != nil {
		_, err := tx.Exec("UPDATE telemetry_sessions SET best_lap_ms = ?, stats_extracted = 1 WHERE id = ?", sessionBestLap(c), sessionID)
		if err != nil {
			return err
		}
	}

	var raceID *int64
	if err := tx.QueryRow("SELECT race_id FROM telemetry_sessions WHERE id = ?", sessionID).Scan(&raceID); err != nil {
		return err
	}
	if raceID != nil {
		if err := syncRaceResults(tx, *raceID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// classStats sono i dati della classifica finale di un'auto che servono alle
// statistiche; nil quando mancano (niente classifica, nessun giro valido).
type classStats struct {
	bestLap, grid, laps, pitStops, penalties, raceTime *int64
}

func statsOf(c *telemetry.FinalClassification, i int) classStats {
	var st classStats
	if c == nil || i >= len(c.Cars) {
		return st
	}
	e := c.Cars[i]
	value := func(v int64) *int64 { return &v }
	if e.BestLapTimeMS > 0 {
		st.bestLap = value(int64(e.BestLapTimeMS))
	}
	if e.GridPosition > 0 {
		st.grid = value(int64(e.GridPosition))
	}
	st.laps, st.pitStops, st.penalties = value(int64(e.NumLaps)), value(int64(e.NumPitStops)), value(int64(e.PenaltiesTime))
	if e.TotalRaceTime > 0 {
		st.raceTime = value(int64(math.Round(e.TotalRaceTime * 1000)))
	}
	return st
}

// sessionBestLap restituisce il miglior giro della sessione tra tutte le auto
// in classifica, IA comprese; nil se nessuno ha completato un giro valido.
func sessionBestLap(c *telemetry.FinalClassification) *int64 {
	var best int64
	for _, e := range c.Cars {
		if ms := int64(e.BestLapTimeMS); ms > 0 && (best == 0 || ms < best) {
			best = ms
		}
	}
	if best == 0 {
		return nil
	}
	return &best
}

// backfillStats legge dalla classifica finale i dati per le statistiche
// (miglior giro, griglia, soste…) delle sessioni registrate prima che il
// server li salvasse. I piloti associati ai giocatori non cambiano.
func (s *Service) backfillStats() {
	type pending struct {
		id        int64
		uid, file string
	}
	rows, err := s.db.Query(`SELECT id, session_uid, file FROM telemetry_sessions
		WHERE has_classification = 1 AND stats_extracted = 0 AND ended_at IS NOT NULL`)
	if err != nil {
		log.Printf("telemetria: %v", err)
		return
	}
	var list []pending
	for rows.Next() {
		var p pending
		if rows.Scan(&p.id, &p.uid, &p.file) == nil {
			list = append(list, p)
		}
	}
	rows.Close()

	for _, p := range list {
		var d sessionData
		uid, err := strconv.ParseUint(p.uid, 16, 64)
		if err == nil {
			d, err = analyzeFile(filepath.Join(s.opts.Dir, p.file), uid)
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("telemetria: lettura della sessione %d: %v", p.id, err)
		}
		if err := s.saveStats(p.id, d.classification); err != nil {
			log.Printf("telemetria: statistiche della sessione %d: %v", p.id, err)
		}
	}
	if len(list) > 0 {
		log.Printf("telemetria: lette le statistiche di %d sessioni registrate", len(list))
	}
}

// saveStats salva i dati della classifica c (nil se non c'è più il file) nei
// giocatori della sessione e aggiorna i risultati della gara collegata.
func (s *Service) saveStats(sessionID int64, c *telemetry.FinalClassification) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if c != nil {
		rows, err := tx.Query("SELECT car_index FROM telemetry_players WHERE session_id = ?", sessionID)
		if err != nil {
			return err
		}
		var cars []int
		for rows.Next() {
			var car int
			if err := rows.Scan(&car); err != nil {
				rows.Close()
				return err
			}
			cars = append(cars, car)
		}
		rows.Close()
		for _, car := range cars {
			st := statsOf(c, car)
			_, err := tx.Exec(`UPDATE telemetry_players SET best_lap_ms = ?, grid_position = ?, laps = ?, pit_stops = ?,
				penalties_s = ?, race_time_ms = ? WHERE session_id = ? AND car_index = ?`,
				st.bestLap, st.grid, st.laps, st.pitStops, st.penalties, st.raceTime, sessionID, car)
			if err != nil {
				return err
			}
		}
		if _, err := tx.Exec("UPDATE telemetry_sessions SET best_lap_ms = ? WHERE id = ?", sessionBestLap(c), sessionID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec("UPDATE telemetry_sessions SET stats_extracted = 1 WHERE id = ?", sessionID); err != nil {
		return err
	}
	raceID, err := sessionRace(tx, sessionID)
	if err != nil {
		return err
	}
	if raceID != nil {
		if err := syncRaceResults(tx, *raceID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// guessPlayer propone il pilota per un giocatore del gioco: quello con lo
// stesso nome, poi quello che nella sessione precedente aveva lo stesso
// controller (in split-screen il nome del gioco non dice chi guida), infine
// quello associato l'ultima volta allo stesso nome. controller è 0 per i
// giocatori senza controller. Restituisce nil se non trova nessuno di libero.
func guessPlayer(db dbtx, sessionID int64, controller int, name string, taken map[int64]bool) (*int64, error) {
	type query struct {
		sql  string
		args []any
	}
	var queries []query
	if name != "" {
		queries = append(queries, query{"SELECT id FROM players WHERE name = ?", []any{name}}) // la colonna è COLLATE NOCASE
	}
	if controller > 0 {
		queries = append(queries, query{`
			SELECT player_id FROM telemetry_players WHERE controller = ? AND player_id IS NOT NULL AND session_id < ?
			ORDER BY session_id DESC LIMIT 1`, []any{controller, sessionID}})
	}
	if name != "" {
		queries = append(queries, query{`
			SELECT player_id FROM telemetry_players WHERE name = ? COLLATE NOCASE AND player_id IS NOT NULL
			ORDER BY session_id DESC LIMIT 1`, []any{name}})
	}
	for _, q := range queries {
		var id int64
		err := db.QueryRow(q.sql, q.args...).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !taken[id] {
			taken[id] = true
			return &id, nil
		}
	}
	return nil, nil
}

// sessionKind raggruppa i tipi di sessione del weekend: "race" (anche le
// sprint), "qualifying" (anche lo Sprint Shootout) e "practice". Per le
// altre sessioni (prova a tempo) restituisce "": non finiscono nei tornei.
// sessionKindSQL fa lo stesso in SQL e va tenuta allineata.
func sessionKind(st telemetry.SessionType) string {
	switch {
	case st.IsRace():
		return "race"
	case st >= telemetry.SessionQualifying1 && st <= telemetry.SessionOneShotSprintShootout:
		return "qualifying"
	case st >= telemetry.SessionPractice1 && st <= telemetry.SessionShortPractice:
		return "practice"
	}
	return ""
}

const sessionKindSQL = `CASE WHEN s.session_type BETWEEN 15 AND 17 THEN 'race'
	WHEN s.session_type BETWEEN 5 AND 14 THEN 'qualifying'
	WHEN s.session_type BETWEEN 1 AND 4 THEN 'practice' END`

// raceKindSQL è il gruppo del tipo di una gara del torneo: GP e sprint sono entrambe "race".
const raceKindSQL = "CASE WHEN r.type IN ('race', 'sprint') THEN 'race' ELSE r.type END"

// sourceSQL seleziona le sessioni s da cui si possono ricavare i risultati della
// gara r: finite, analizzate e dello stesso tipo (una qualifica non dà i risultati di un GP).
const sourceSQL = "s.has_classification = 1 AND s.analyzed = 1 AND " + sessionKindSQL + " = " + raceKindSQL

// resultsSource restituisce la registrazione da cui derivano i risultati della gara, o 0.
func resultsSource(db dbtx, raceID int64) (int64, error) {
	var id int64
	err := db.QueryRow(`
		SELECT s.id FROM telemetry_sessions s JOIN races r ON r.id = s.race_id
		WHERE r.id = ? AND `+sourceSQL+" ORDER BY s.id DESC LIMIT 1", raceID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// isSourceFor dice se la sessione può dare i risultati della gara.
func isSourceFor(db dbtx, sessionID, raceID int64) (bool, error) {
	var ok bool
	// Per le sessioni fuori dai tornei il confronto dei tipi vale NULL.
	err := db.QueryRow("SELECT COALESCE("+sourceSQL+", 0) FROM telemetry_sessions s, races r WHERE s.id = ? AND r.id = ?",
		sessionID, raceID).Scan(&ok)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return ok, err
}

// raceTypeFor restituisce il tipo della gara del torneo per una sessione del
// tipo st, "" se la sessione non va nei tornei. Le sprint si riconoscono solo
// dalla struttura del weekend, che qui non c'è: vedi isSprint.
func raceTypeFor(st telemetry.SessionType) string {
	return sessionKind(st) // "race", "qualifying", "practice" sono anche tipi di gara
}

// syncRaceResults riscrive i risultati della gara a partire dalla sua
// registrazione: ogni pilota associato a un giocatore prende la posizione
// d'arrivo di quell'auto. La classifica del torneo si calcola dai risultati,
// quindi si aggiorna da sola. Se la gara non ha registrazioni con la
// classifica finale, o se i suoi risultati sono stati modificati a mano,
// restano com'erano.
func syncRaceResults(db dbtx, raceID int64) error {
	var manual bool
	err := db.QueryRow("SELECT manual_results FROM races WHERE id = ?", raceID).Scan(&manual)
	if errors.Is(err, sql.ErrNoRows) || manual {
		return nil // gara eliminata, o risultati da non toccare finché non si ripristinano (ResyncRace)
	}
	if err != nil {
		return err
	}
	sessionID, err := resultsSource(db, raceID)
	if err != nil || sessionID == 0 {
		return err
	}
	// Il giro più veloce della gara è di chi ha fatto il miglior giro della
	// sessione, tra tutte le auto (IA comprese).
	var sessionBest *int64
	if err := db.QueryRow("SELECT best_lap_ms FROM telemetry_sessions WHERE id = ?", sessionID).Scan(&sessionBest); err != nil {
		return err
	}
	rows, err := db.Query(`SELECT player_id, position, result_status, best_lap_ms FROM telemetry_players
		WHERE session_id = ? AND player_id IS NOT NULL`, sessionID)
	if err != nil {
		return err
	}
	type result struct {
		playerID         int64
		position, status *int
		bestLap          *int64
	}
	var results []result
	for rows.Next() {
		var r result
		if err := rows.Scan(&r.playerID, &r.position, &r.status, &r.bestLap); err != nil {
			rows.Close()
			return err
		}
		results = append(results, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	if _, err := db.Exec("DELETE FROM results WHERE race_id = ?", raceID); err != nil {
		return err
	}
	for _, r := range results {
		position, dnf := r.position, !classified(r.status)
		if dnf {
			position = nil
		}
		fastest := r.bestLap != nil && sessionBest != nil && *r.bestLap == *sessionBest
		_, err := db.Exec("INSERT INTO results (race_id, player_id, position, dnf, best_lap_ms, fastest_lap) VALUES (?, ?, ?, ?, ?, ?)",
			raceID, r.playerID, position, dnf, r.bestLap, fastest)
		if err != nil {
			return err
		}
	}
	return nil
}

// classified dice se un'auto è arrivata al traguardo (o era ancora in pista quando è finita la gara).
func classified(status *int) bool {
	return status != nil &&
		(telemetry.ResultStatus(*status) == telemetry.ResultFinished || telemetry.ResultStatus(*status) == telemetry.ResultActive)
}

// --- Modifiche dalla pagina delle registrazioni ----------------------------

// SetSessionPlayers cambia il pilota associato ai giocatori della sessione
// (chiave: indice dell'auto; nil toglie l'associazione) e aggiorna i
// risultati della gara collegata.
func (s *Service) SetSessionPlayers(sessionID int64, players map[int]*int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	raceID, err := sessionRace(tx, sessionID)
	if err != nil {
		return err
	}
	var file string
	if err := tx.QueryRow("SELECT file FROM telemetry_sessions WHERE id = ?", sessionID).Scan(&file); err != nil {
		return err
	}
	for carIndex, playerID := range players {
		if playerID != nil {
			var one int
			err := tx.QueryRow("SELECT 1 FROM players WHERE id = ?", *playerID).Scan(&one)
			if errors.Is(err, sql.ErrNoRows) {
				return invalid("Pilota %d inesistente", *playerID)
			}
			if err != nil {
				return err
			}
		}
		res, err := tx.Exec("UPDATE telemetry_players SET player_id = ? WHERE session_id = ? AND car_index = ?",
			playerID, sessionID, carIndex)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return invalid("Nella sessione non c'è un giocatore sull'auto %d", carIndex)
		}
	}

	// Il controllo si fa dopo gli aggiornamenti, così si possono anche scambiare due piloti.
	var dup string
	err = tx.QueryRow(`
		SELECT p.name FROM telemetry_players tp JOIN players p ON p.id = tp.player_id
		WHERE tp.session_id = ? GROUP BY tp.player_id HAVING COUNT(*) > 1 LIMIT 1`, sessionID).Scan(&dup)
	if err == nil {
		return invalid("%s è associato a più giocatori della stessa sessione", dup)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	if raceID != nil {
		if err := syncRaceResults(tx, *raceID); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	// Il replay mostra i nomi dei piloti: va ripreparato.
	s.dropReplay(sessionID, file)
	return nil
}

// SetSessionRace collega la sessione a una gara (nil la scollega).
func (s *Service) SetSessionRace(sessionID int64, raceID *int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if raceID != nil {
		var one int
		err := tx.QueryRow("SELECT 1 FROM races WHERE id = ?", *raceID).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return invalid("Gara inesistente")
		}
		if err != nil {
			return err
		}
	} else {
		var finished bool
		err := tx.QueryRow("SELECT COALESCE(s.has_classification = 1 AND "+sessionKindSQL+" IS NOT NULL, 0) FROM telemetry_sessions s WHERE s.id = ?",
			sessionID).Scan(&finished)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrSessionNotFound
		}
		if err != nil {
			return err
		}
		if finished {
			return invalid("Le sessioni finite stanno sempre in un torneo: per toglierla da qui spostala in \"Nessun torneo\"")
		}
	}
	if err := linkRace(tx, sessionID, raceID); err != nil {
		return err
	}
	return tx.Commit()
}

// LinkNewRace crea nel torneo una gara sul circuito della sessione e la collega.
// raceType è "race", "sprint", "qualifying" o "practice"; vuoto per ricavarlo
// dal tipo di sessione.
func (s *Service) LinkNewRace(sessionID, tournamentID int64, raceType string) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	raceID, err := linkNewRace(tx, sessionID, tournamentID, raceType)
	if err != nil {
		return 0, err
	}
	return raceID, tx.Commit()
}

// linkNewRace è LinkNewRace dentro una transazione già aperta.
func linkNewRace(tx dbtx, sessionID, tournamentID int64, raceType string) (int64, error) {
	spec, err := raceSpecOf(tx, sessionID, raceType)
	if err != nil {
		return 0, err
	}
	var one int
	err = tx.QueryRow("SELECT 1 FROM tournaments WHERE id = ?", tournamentID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, invalid("Torneo inesistente")
	}
	if err != nil {
		return 0, err
	}
	var raceID int64
	err = tx.QueryRow("INSERT INTO races (tournament_id, track, track_id, type, date) VALUES (?, ?, ?, ?, ?) RETURNING id",
		tournamentID, spec.track, spec.trackID, spec.kind, spec.date).Scan(&raceID)
	if err != nil {
		return 0, err
	}
	if err := linkRace(tx, sessionID, &raceID); err != nil {
		return 0, err
	}
	return raceID, nil
}

// raceSpec è la gara del torneo che corrisponde a una sessione registrata.
type raceSpec struct {
	trackID *int
	track   string  // nome del circuito, come per le gare inserite a mano
	kind    string  // tipo della gara: "race", "sprint", "qualifying" o "practice"
	date    *string // giorno della sessione, nil se non si sa
}

// raceSpecOf ricava circuito, tipo e data della gara per la sessione. raceType
// vuoto (o non valido) si ricava dal tipo di sessione.
func raceSpecOf(tx dbtx, sessionID int64, raceType string) (raceSpec, error) {
	var (
		spec        raceSpec
		sessionType *int
		startedAt   string
	)
	err := tx.QueryRow("SELECT track_id, session_type, started_at FROM telemetry_sessions WHERE id = ?", sessionID).
		Scan(&spec.trackID, &sessionType, &startedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return spec, ErrSessionNotFound
	}
	if err != nil {
		return spec, err
	}
	spec.kind = raceType
	switch raceType {
	case "race", "sprint", "qualifying", "practice":
	default:
		spec.kind = "race"
		if sessionType != nil && raceTypeFor(telemetry.SessionType(*sessionType)) != "" {
			spec.kind = raceTypeFor(telemetry.SessionType(*sessionType))
		}
	}
	spec.track = "Circuito sconosciuto"
	if spec.trackID != nil {
		spec.track = RaceTrackName(telemetry.TrackID(*spec.trackID))
	}
	// La data della gara è quella locale, come la inserirebbe l'utente a mano.
	if t, err := time.Parse("2006-01-02 15:04:05", startedAt); err == nil {
		d := t.Local().Format("2006-01-02")
		spec.date = &d
	}
	return spec, nil
}

// linkRace sposta la sessione su un'altra gara (o nessuna, con nil) e
// aggiorna i risultati di entrambe le gare.
func linkRace(tx dbtx, sessionID int64, raceID *int64) error {
	oldRace, err := sessionRace(tx, sessionID)
	if err != nil {
		return err
	}
	moved := raceID != nil && (oldRace == nil || *oldRace != *raceID) // arriva in una gara diversa
	source := false
	if raceID != nil {
		if source, err = isSourceFor(tx, sessionID, *raceID); err != nil {
			return err
		}
		if source {
			current, err := resultsSource(tx, *raceID)
			if err != nil {
				return err
			}
			if current != 0 && current != sessionID {
				return invalid("La gara ha già una registrazione con la classifica finale: spostala prima in un'altra gara")
			}
		}
	}
	wasSource := false
	if oldRace != nil && (raceID == nil || *raceID != *oldRace) {
		if wasSource, err = isSourceFor(tx, sessionID, *oldRace); err != nil {
			return err
		}
	}

	if _, err := tx.Exec("UPDATE telemetry_sessions SET race_id = ? WHERE id = ?", raceID, sessionID); err != nil {
		return err
	}
	if raceID != nil {
		// Assegnata a una gara: non aspetta più di essere salvata (vedi recoverPending).
		if _, err := tx.Exec("UPDATE telemetry_sessions SET pending_save = 0 WHERE id = ?", sessionID); err != nil {
			return err
		}
	}
	if source && moved {
		// I risultati della gara vengono da questa registrazione e sostituiscono
		// quelli inseriti prima, anche se modificati a mano.
		if _, err := tx.Exec("UPDATE races SET manual_results = 0 WHERE id = ?", *raceID); err != nil {
			return err
		}
	}
	if wasSource {
		// I risultati della gara di prima venivano da questa registrazione: non valgono più.
		if _, err := tx.Exec("UPDATE races SET manual_results = 0 WHERE id = ?", *oldRace); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM results WHERE race_id = ?", *oldRace); err != nil {
			return err
		}
		if err := syncRaceResults(tx, *oldRace); err != nil {
			return err
		}
		// La gara di prima, se è in "Nessun torneo" ed è rimasta vuota, non serve più:
		// di solito è quella creata in automatico prima di spostare la sessione nel torneo giusto.
		_, err := tx.Exec(`
			DELETE FROM races WHERE id = ?
			AND tournament_id IN (SELECT id FROM tournaments WHERE free = 1)
			AND NOT EXISTS (SELECT 1 FROM telemetry_sessions WHERE race_id = races.id)
			AND NOT EXISTS (SELECT 1 FROM results WHERE race_id = races.id)`, *oldRace)
		if err != nil {
			return err
		}
	}
	if raceID != nil {
		return syncRaceResults(tx, *raceID)
	}
	return nil
}

// sessionRace restituisce la gara collegata alla sessione (nil se nessuna).
func sessionRace(db dbtx, sessionID int64) (*int64, error) {
	var raceID *int64
	err := db.QueryRow("SELECT race_id FROM telemetry_sessions WHERE id = ?", sessionID).Scan(&raceID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	return raceID, err
}

// DeleteSession elimina la registrazione e il suo file. I risultati della
// gara collegata restano: la classifica del torneo non cambia.
func (s *Service) DeleteSession(sessionID int64) error {
	s.mu.Lock()
	recording := s.current != nil && s.current.id == sessionID
	s.mu.Unlock()
	if recording {
		return ErrSessionRecording
	}

	var file string
	err := s.db.QueryRow("SELECT file FROM telemetry_sessions WHERE id = ?", sessionID).Scan(&file)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSessionNotFound
	}
	if err != nil {
		return err
	}
	return s.removeSession(sessionID, file)
}

func (s *Service) removeSession(sessionID int64, file string) error {
	s.dropReplay(sessionID, file)
	if err := os.Remove(filepath.Join(s.opts.Dir, file)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_, err := s.db.Exec("DELETE FROM telemetry_sessions WHERE id = ?", sessionID)
	return err
}

// unfinishedSQL seleziona le sessioni finite senza classifica finale e non
// assegnate a un torneo: non servono alla classifica e vengono eliminate.
const unfinishedSQL = "has_classification = 0 AND race_id IS NULL AND ended_at IS NOT NULL"

// deleteUnfinished elimina le sessioni non terminate più vecchie di unfinishedRetention.
func (s *Service) deleteUnfinished(now time.Time) {
	s.removeUnfinished(now.Add(-unfinishedRetention))
}

// DeleteUnfinished elimina subito tutte le sessioni non terminate, senza
// aspettare unfinishedRetention: restituisce quante erano e quanto spazio
// occupavano i loro file.
func (s *Service) DeleteUnfinished() (deleted int, freed int64, err error) {
	return s.removeUnfinished(time.Now())
}

// UnfinishedUsage restituisce quante sessioni non terminate ci sono e quanto
// spazio occupano i loro file: quello che libera DeleteUnfinished.
func (s *Service) UnfinishedUsage() (count int, size int64, err error) {
	list, err := s.unfinished(time.Now())
	for _, o := range list {
		count++
		size += s.filesSize(o.file)
	}
	return count, size, err
}

type unfinishedSession struct {
	id   int64
	file string
}

// unfinished restituisce le sessioni non terminate finite prima di before.
func (s *Service) unfinished(before time.Time) ([]unfinishedSession, error) {
	// Gli orari sono testi nello stesso formato: il confronto alfabetico è anche cronologico.
	rows, err := s.db.Query("SELECT id, file FROM telemetry_sessions WHERE "+unfinishedSQL+" AND ended_at < ?", dbTime(before))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []unfinishedSession
	for rows.Next() {
		var o unfinishedSession
		if err := rows.Scan(&o.id, &o.file); err != nil {
			return nil, err
		}
		list = append(list, o)
	}
	return list, rows.Err()
}

// removeUnfinished elimina le sessioni non terminate finite prima di before.
func (s *Service) removeUnfinished(before time.Time) (deleted int, freed int64, err error) {
	list, err := s.unfinished(before)
	if err != nil {
		log.Printf("telemetria: %v", err)
		return 0, 0, err
	}
	for _, o := range list {
		size := s.filesSize(o.file)
		if err := s.removeSession(o.id, o.file); err != nil {
			log.Printf("telemetria: eliminazione della sessione %d: %v", o.id, err)
			continue
		}
		deleted++
		freed += size
	}
	if deleted > 0 {
		log.Printf("telemetria: eliminate %d sessioni non terminate (%.1f MB)", deleted, float64(freed)/(1<<20))
		s.DataChanged()
	}
	return deleted, freed, nil
}

// filesSize restituisce quanto occupano la registrazione e il suo replay salvato.
func (s *Service) filesSize(file string) int64 {
	var size int64
	for _, name := range []string{file, file + replayExt} {
		if info, err := os.Stat(filepath.Join(s.opts.Dir, name)); err == nil {
			size += info.Size()
		}
	}
	return size
}
