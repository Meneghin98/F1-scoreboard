package live

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
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
	header         telemetry.Header // l'ultimo: contiene gli indici delle auto dei giocatori
	participants   *telemetry.Participants
	classification *telemetry.FinalClassification
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
		// Si decodificano solo i due pacchetti utili: sono pochi rispetto al totale.
		if h.PacketID != telemetry.PacketParticipants && h.PacketID != telemetry.PacketFinalClassification {
			continue
		}
		packet, err := telemetry.Parse(p.Data)
		if err != nil {
			continue
		}
		d.header = h
		switch packet := packet.(type) {
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
		n := int(p.NumActiveCars)
		cars = &n
		taken := map[int64]bool{} // un pilota non può guidare due auto nella stessa gara
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
			playerID, err := guessPlayer(tx, car.Name, taken)
			if err != nil {
				return err
			}
			_, err = tx.Exec(`
				INSERT INTO telemetry_players (session_id, car_index, name, team, race_number, position, result_status, player_id)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				sessionID, i, car.Name, car.TeamID.String(), int(car.RaceNumber), position, status, playerID)
			if err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec("UPDATE telemetry_sessions SET analyzed = 1, cars = ? WHERE id = ?", cars, sessionID); err != nil {
		return err
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

// guessPlayer propone il pilota per un giocatore del gioco: quello con lo
// stesso nome, oppure quello associato l'ultima volta allo stesso nome.
// Restituisce nil se non trova nessuno di libero.
func guessPlayer(db dbtx, name string, taken map[int64]bool) (*int64, error) {
	queries := []string{
		"SELECT id FROM players WHERE name = ?", // la colonna è COLLATE NOCASE
		`SELECT player_id FROM telemetry_players WHERE name = ? COLLATE NOCASE AND player_id IS NOT NULL
		 ORDER BY session_id DESC LIMIT 1`,
	}
	for _, q := range queries {
		var id int64
		err := db.QueryRow(q, name).Scan(&id)
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

// raceSessionTypes sono i tipi di sessione che danno punti (vedi telemetry.SessionType.IsRace).
var raceSessionTypes = []any{int(telemetry.SessionRace), int(telemetry.SessionRace2), int(telemetry.SessionRace3)}

// resultsSourceSQL seleziona le sessioni da cui si possono ricavare i risultati di una gara.
const resultsSourceSQL = "has_classification = 1 AND analyzed = 1 AND session_type IN (?, ?, ?)"

// resultsSource restituisce la registrazione da cui derivano i risultati della gara, o 0.
func resultsSource(db dbtx, raceID int64) (int64, error) {
	var id int64
	args := append([]any{raceID}, raceSessionTypes...) // nuova slice: raceSessionTypes non cambia
	err := db.QueryRow("SELECT id FROM telemetry_sessions WHERE race_id = ? AND "+resultsSourceSQL+" ORDER BY id DESC LIMIT 1", args...).
		Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// syncRaceResults riscrive i risultati della gara a partire dalla sua
// registrazione: ogni pilota associato a un giocatore prende la posizione
// d'arrivo di quell'auto. La classifica del torneo si calcola dai risultati,
// quindi si aggiorna da sola. Se la gara non ha registrazioni con la
// classifica finale, i risultati inseriti a mano restano com'erano.
func syncRaceResults(db dbtx, raceID int64) error {
	sessionID, err := resultsSource(db, raceID)
	if err != nil || sessionID == 0 {
		return err
	}
	rows, err := db.Query(
		"SELECT player_id, position, result_status FROM telemetry_players WHERE session_id = ? AND player_id IS NOT NULL",
		sessionID)
	if err != nil {
		return err
	}
	type result struct {
		playerID         int64
		position, status *int
	}
	var results []result
	for rows.Next() {
		var r result
		if err := rows.Scan(&r.playerID, &r.position, &r.status); err != nil {
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
		_, err := db.Exec("INSERT INTO results (race_id, player_id, position, dnf) VALUES (?, ?, ?, ?)",
			raceID, r.playerID, position, dnf)
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
	return tx.Commit()
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
	}
	if err := linkRace(tx, sessionID, raceID); err != nil {
		return err
	}
	return tx.Commit()
}

// LinkNewRace crea nel torneo una gara sul circuito della sessione e la collega.
// raceType è "race" o "sprint".
func (s *Service) LinkNewRace(sessionID, tournamentID int64, raceType string) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var (
		trackID   *int
		startedAt string
	)
	err = tx.QueryRow("SELECT track_id, started_at FROM telemetry_sessions WHERE id = ?", sessionID).Scan(&trackID, &startedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrSessionNotFound
	}
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
	if raceType != "sprint" {
		raceType = "race"
	}
	track := "Circuito sconosciuto"
	if trackID != nil {
		track = RaceTrackName(telemetry.TrackID(*trackID))
	}
	// La data della gara è quella locale, come la inserirebbe l'utente a mano.
	var date *string
	if t, err := time.Parse("2006-01-02 15:04:05", startedAt); err == nil {
		d := t.Local().Format("2006-01-02")
		date = &d
	}

	var raceID int64
	err = tx.QueryRow("INSERT INTO races (tournament_id, track, type, date) VALUES (?, ?, ?, ?) RETURNING id",
		tournamentID, track, raceType, date).Scan(&raceID)
	if err != nil {
		return 0, err
	}
	if err := linkRace(tx, sessionID, &raceID); err != nil {
		return 0, err
	}
	return raceID, tx.Commit()
}

// linkRace sposta la sessione su un'altra gara (o nessuna, con nil) e
// aggiorna i risultati di entrambe le gare.
func linkRace(tx dbtx, sessionID int64, raceID *int64) error {
	oldRace, err := sessionRace(tx, sessionID)
	if err != nil {
		return err
	}
	// Si copia la slice: append su raceSessionTypes potrebbe scriverci dentro.
	args := append(append([]any{}, raceSessionTypes...), sessionID)
	var isSource bool
	if err := tx.QueryRow("SELECT "+resultsSourceSQL+" FROM telemetry_sessions WHERE id = ?", args...).
		Scan(&isSource); err != nil {
		return err
	}

	if raceID != nil && isSource {
		source, err := resultsSource(tx, *raceID)
		if err != nil {
			return err
		}
		if source != 0 && source != sessionID {
			return invalid("La gara ha già una registrazione con la classifica finale: scollegala prima di collegarne un'altra")
		}
	}

	if _, err := tx.Exec("UPDATE telemetry_sessions SET race_id = ? WHERE id = ?", raceID, sessionID); err != nil {
		return err
	}
	// I risultati della gara di prima venivano da questa registrazione: non valgono più.
	if oldRace != nil && (raceID == nil || *raceID != *oldRace) && isSource {
		if _, err := tx.Exec("DELETE FROM results WHERE race_id = ?", *oldRace); err != nil {
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
	// Gli orari sono testi nello stesso formato: il confronto alfabetico è anche cronologico.
	rows, err := s.db.Query("SELECT id, file FROM telemetry_sessions WHERE "+unfinishedSQL+" AND ended_at < ?",
		dbTime(now.Add(-unfinishedRetention)))
	if err != nil {
		log.Printf("telemetria: %v", err)
		return
	}
	type old struct {
		id   int64
		file string
	}
	var list []old
	for rows.Next() {
		var o old
		if rows.Scan(&o.id, &o.file) == nil {
			list = append(list, o)
		}
	}
	rows.Close()

	deleted := 0
	for _, o := range list {
		if err := s.removeSession(o.id, o.file); err != nil {
			log.Printf("telemetria: eliminazione della sessione %d: %v", o.id, err)
			continue
		}
		deleted++
	}
	if deleted > 0 {
		log.Printf("telemetria: eliminate %d sessioni non terminate", deleted)
	}
}
