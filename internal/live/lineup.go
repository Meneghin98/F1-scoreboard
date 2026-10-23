package live

import (
	"database/sql"
	"errors"
	"log"
	"math"
	"slices"
	"time"

	"f1-scoreboard/internal/telemetry"
)

// Chi guida? In split-screen il gioco dice quale auto ha ciascun controller,
// ma non chi lo tiene in mano. In ogni sessione del weekend (prove libere,
// qualifiche, gara) il server propone i piloti (quelli dell'ultima volta) e la
// pagina Live permette di cambiarli; a fine sessione chiede di confermare i
// risultati, poi la salva nel torneo in corso o, se non ce n'è uno, in
// "Nessun torneo". Prove libere e qualifiche non danno punti.
//
// Se nessuno interviene, la scelta dei piloti si chiude da sola alla partenza
// (o dopo lineupTimeout) e i risultati si salvano dopo lineupTimeout: i punti
// arrivano in classifica anche se nessuno guarda la pagina. Se qualcuno
// modifica i piloti il conto alla rovescia si ferma e si aspetta la sua
// conferma, ma se la sessione finisce prima i risultati vengono salvati comunque.

const (
	lineupTimeout = 5 * time.Minute
	// Se il salvataggio dei risultati non riesce (per esempio il database è
	// occupato) si riprova dopo questo tempo; intanto la pagina Live mostra l'errore.
	saveRetry = 30 * time.Second
	// Dopo il salvataggio di una sessione la pagina Live ne mostra il riepilogo
	// per questo tempo, o finché non parte la sessione successiva.
	lastSavedFor = 15 * time.Minute
)

// Fasi della scelta dei piloti.
const (
	StageDrivers = "drivers" // prima della partenza: chi guida?
	StageRacing  = "racing"  // sessione in corso: i piloti si possono ancora cambiare
	StageResults = "results" // classifica finale arrivata: si confermano i risultati
	StageSaved   = "saved"   // risultati salvati nel torneo
)

// lineup è lo stato della scelta dei piloti per la sessione in corso.
type lineup struct {
	stage    string
	cars     []lineupCar
	deadline time.Time // salvataggio automatico; zero se non previsto o se l'utente è intervenuto
	saved    *LineupTournament
	raceID   *int64
	saveErr  string // perché l'ultimo salvataggio non è riuscito, vuoto se non ci sono stati errori
}

type lineupCar struct {
	controller  int // 1 o 2
	carIndex    int
	playerID    *int64
	playerName  string
	playerColor string
}

// Lineup è la scelta dei piloti come arriva al frontend.
type Lineup struct {
	SessionID int64  `json:"sessionId"` // riga in telemetry_sessions: la conferma vale solo per questa sessione
	Stage     string `json:"stage"`
	Kind      string `json:"kind"` // "race", "qualifying" o "practice", vedi sessionKind
	// SecondsLeft sono i secondi al salvataggio automatico, nil se non è previsto.
	SecondsLeft *int        `json:"secondsLeft"`
	Cars        []LineupCar `json:"cars"`
	// Tournament è il torneo in cui finirà la gara (o in cui è finita, se salvata).
	Tournament *LineupTournament `json:"tournament"`
	RaceID     *int64            `json:"raceId"` // gara creata nel torneo, dopo il salvataggio
	// Error dice perché il salvataggio dei risultati non è riuscito (si riprova
	// da soli dopo poco); vuoto se non ci sono stati errori.
	Error string `json:"error"`
}

type LineupCar struct {
	Controller  int    `json:"controller"`
	CarIndex    int    `json:"carIndex"`
	Name        string `json:"name"` // nome mostrato dal gioco
	Team        string `json:"team"`
	RaceNumber  uint8  `json:"raceNumber"`
	PlayerID    *int64 `json:"playerId"`
	PlayerName  string `json:"playerName"`
	PlayerColor string `json:"playerColor"` // colore del pilota, vuoto se non scelto
	Position    *int   `json:"position"`    // dalla classifica finale
	DNF         bool   `json:"dnf"`
}

type LineupTournament struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Free bool   `json:"free"`
}

// SavedLineup è il riepilogo dell'ultima sessione salvata nel torneo: la
// pagina Live lo mostra anche quando è già iniziata la successiva, così si
// vede dove è finita e si possono correggere i piloti.
type SavedLineup struct {
	Lineup
	Track   string `json:"track"`
	Session string `json:"session"` // tipo della sessione, per esempio "Gara"
	until   time.Time
}

// ErrNoLineup indica che non c'è una gara in corso di cui scegliere i piloti.
var ErrNoLineup = errors.New("nessuna gara in corso")

// updateLineup fa avanzare la scelta dei piloti in base alla telemetria e alle scadenze.
func (s *Service) updateLineup(at time.Time) {
	cur := s.current
	if cur == nil {
		return
	}
	if cur.lineup == nil {
		s.startLineup(cur, at)
		if cur.lineup == nil {
			return
		}
	}
	l := cur.lineup
	if l.stage == StageDrivers && (s.tracker.RaceStarted() || expired(l, at)) {
		l.stage, l.deadline = StageRacing, time.Time{}
	}
	if l.stage != StageDrivers {
		s.lastSaved = nil // la sessione successiva è partita: il riepilogo della precedente non serve più
	}
	if l.stage == StageRacing && s.tracker.Classification() != nil {
		l.stage, l.deadline = StageResults, at.Add(lineupTimeout)
	}
	if l.stage == StageResults && expired(l, at) {
		if err := s.saveLineup(cur, at); err != nil {
			log.Printf("telemetria: salvataggio della gara: %v", err)
		}
	}
}

func expired(l *lineup, at time.Time) bool {
	return !l.deadline.IsZero() && !at.Before(l.deadline)
}

// startLineup prepara la scelta dei piloti appena si sa che la sessione è del
// weekend e chi sono i giocatori. Per le altre sessioni (prova a tempo) non fa nulla.
func (s *Service) startLineup(cur *session, at time.Time) {
	ses, parts := s.tracker.Session(), s.tracker.Participants()
	if ses == nil || sessionKind(ses.SessionType) == "" || (parts == nil && s.tracker.Classification() == nil) {
		return
	}
	l := &lineup{stage: StageDrivers, deadline: at.Add(lineupTimeout)}
	h := s.tracker.header
	taken := map[int64]bool{}
	for i, idx := range []uint8{h.PlayerCarIndex, h.SecondaryPlayerCarIndex} {
		if idx == 255 { // nessun giocatore (spettatore, o niente split-screen)
			continue
		}
		car := lineupCar{controller: i + 1, carIndex: int(idx)}
		var name string
		if parts != nil && int(idx) < len(parts.Cars) {
			name = parts.Cars[idx].Name
		}
		id, err := guessPlayer(s.db, cur.id, car.controller, name, taken)
		if err != nil {
			log.Printf("telemetria: scelta dei piloti: %v", err)
		}
		if id != nil {
			car.playerID = id
			if p, err := playerInfo(s.db, *id); err != nil {
				log.Printf("telemetria: %v", err)
			} else {
				car.playerName, car.playerColor = p.Name, p.Color
			}
		}
		l.cars = append(l.cars, car)
	}
	cur.lineup = l
}

// scoreboardPlayer è un pilota del segnapunti come lo mostrano live e replay.
type scoreboardPlayer struct {
	Name, Color string
}

// playerInfo restituisce nome e colore del pilota id (sql.ErrNoRows se non esiste).
func playerInfo(db dbtx, id int64) (scoreboardPlayer, error) {
	var p scoreboardPlayer
	err := db.QueryRow("SELECT name, COALESCE(color, '') FROM players WHERE id = ?", id).Scan(&p.Name, &p.Color)
	return p, err
}

func (l *lineup) assigned() map[int]*int64 {
	m := make(map[int]*int64, len(l.cars))
	for _, c := range l.cars {
		m[c.carIndex] = c.playerID
	}
	return m
}

// saveLineup salva i giocatori della sessione con i piloti scelti e, se la
// gara ha la classifica finale, la aggiunge al torneo in corso (o in "Nessun
// torneo"). Se qualcosa va storto la fase resta quella dei risultati: si
// riprova da soli dopo saveRetry, oppure confermando di nuovo.
func (s *Service) saveLineup(cur *session, at time.Time) error {
	l := cur.lineup
	if err := s.storeLineup(cur); err != nil {
		l.saveErr, l.deadline = err.Error(), at.Add(saveRetry)
		return err
	}
	l.stage, l.deadline, l.saveErr = StageSaved, time.Time{}, ""
	s.dataChangedLocked() // la gara è nel torneo: classifica e gare vanno ricaricate
	return nil
}

func (s *Service) storeLineup(cur *session) error {
	l := cur.lineup
	d := s.sessionData()
	d.assigned = l.assigned()
	if err := s.savePlayers(cur.id, d); err != nil {
		return err
	}
	cur.playersSaved = true

	ses := s.tracker.Session()
	if ses == nil || d.classification == nil {
		return nil
	}
	t, raceID, err := s.addRaceToTournament(cur.id, raceTypeOf(ses))
	if err != nil {
		return err
	}
	l.saved, l.raceID = t, raceID
	if t != nil {
		log.Printf("telemetria: sessione %016x: gara salvata nel torneo %q", cur.uid, t.Name)
	}
	return nil
}

// sessionData raccoglie dal tracker i dati della sessione in corso.
func (s *Service) sessionData() sessionData {
	return sessionData{header: s.tracker.header, participants: s.tracker.Participants(), classification: s.tracker.Classification()}
}

// addRaceToTournament mette la sessione in una gara del torneo in corso (o di
// "Nessun torneo") e restituisce torneo e gara: quella in calendario sullo
// stesso circuito, se c'è, altrimenti una nuova. Se la sessione è già
// collegata a una gara (a mano, dalla pagina delle registrazioni) resta dov'è.
// Tutto avviene in una transazione: chiamata due volte per la stessa sessione
// (dal salvataggio e dal recupero dopo un'interruzione) crea una gara sola.
func (s *Service) addRaceToTournament(sessionID int64, raceType string) (*LineupTournament, *int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	raceID, err := sessionRace(tx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	if raceID == nil {
		target, err := targetTournament(tx)
		if err != nil || target == nil {
			return nil, nil, err
		}
		spec, err := raceSpecOf(tx, sessionID, raceType)
		if err != nil {
			return nil, nil, err
		}
		id, err := plannedRace(tx, target.ID, spec)
		if err != nil {
			return nil, nil, err
		}
		if id != 0 {
			// La gara era in calendario: prende il giorno in cui si è corsa davvero.
			_, err = tx.Exec("UPDATE races SET date = COALESCE(?, date), track_id = COALESCE(track_id, ?) WHERE id = ?",
				spec.date, spec.trackID, id)
			if err == nil {
				err = linkRace(tx, sessionID, &id)
			}
		} else {
			id, err = linkNewRace(tx, sessionID, target.ID, spec.kind)
		}
		if err != nil {
			return nil, nil, err
		}
		raceID = &id
	}
	// La sessione è in un torneo: non aspetta più di essere salvata (vedi recoverPending).
	if _, err := tx.Exec("UPDATE telemetry_sessions SET pending_save = 0 WHERE id = ?", sessionID); err != nil {
		return nil, nil, err
	}
	var t LineupTournament
	err = tx.QueryRow(`
		SELECT t.id, t.name, t.free FROM races r JOIN tournaments t ON t.id = r.tournament_id WHERE r.id = ?`, *raceID).
		Scan(&t.ID, &t.Name, &t.Free)
	if err != nil {
		return nil, nil, err
	}
	return &t, raceID, tx.Commit()
}

// raceTypeOf restituisce il tipo della gara del torneo per la sessione ses:
// come raceTypeFor, ma riconosce anche le sprint dalla struttura del weekend.
func raceTypeOf(ses *telemetry.Session) string {
	if isSprint(ses) {
		return "sprint"
	}
	return raceTypeFor(ses.SessionType)
}

// isSprint dice se la gara è una sprint: nei weekend con la sprint (quelli con
// lo Sprint Shootout) la sprint è la prima gara del weekend.
func isSprint(ses *telemetry.Session) bool {
	sprintWeekend := slices.ContainsFunc(ses.WeekendStructure, func(t telemetry.SessionType) bool {
		return t >= telemetry.SessionSprintShootout1 && t <= telemetry.SessionOneShotSprintShootout
	})
	if !sprintWeekend {
		return false
	}
	first := slices.IndexFunc(ses.WeekendStructure, telemetry.SessionType.IsRace)
	return first >= 0 && ses.WeekendStructure[first] == ses.SessionType
}

// targetTournament restituisce il torneo in cui finiscono le gare: quello in
// corso o, se non ce n'è uno, "Nessun torneo". nil se non esiste nemmeno quello.
func targetTournament(db dbtx) (*LineupTournament, error) {
	var t LineupTournament
	err := db.QueryRow(`
		SELECT id, name, free FROM tournaments
		WHERE (started_at IS NOT NULL AND ended_at IS NULL) OR free = 1
		ORDER BY free LIMIT 1`).Scan(&t.ID, &t.Name, &t.Free)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// lineupView prepara la scelta dei piloti per il frontend e scrive i nomi dei
// piloti scelti nella classifica live.
func (s *Service) lineupView(snap *Snapshot, now time.Time) {
	cur := s.current
	if cur == nil || cur.lineup == nil {
		return
	}
	v := s.lineupOf(cur, now)
	for _, c := range cur.lineup.cars {
		for i := range snap.Drivers {
			if snap.Drivers[i].CarIndex == c.carIndex {
				snap.Drivers[i].PlayerName, snap.Drivers[i].PlayerColor = c.playerName, c.playerColor
			}
		}
	}
	snap.Lineup = v
}

// lineupOf restituisce la scelta dei piloti della sessione cur come arriva al frontend.
func (s *Service) lineupOf(cur *session, now time.Time) *Lineup {
	l := cur.lineup
	v := &Lineup{
		SessionID: cur.id, Stage: l.stage, Kind: sessionKind(s.tracker.Session().SessionType), Cars: []LineupCar{},
		Tournament: l.saved, RaceID: l.raceID, Error: l.saveErr,
	}
	if !l.deadline.IsZero() {
		secs := max(0, int(math.Ceil(l.deadline.Sub(now).Seconds())))
		v.SecondsLeft = &secs
	}
	if v.Tournament == nil {
		if !s.targetLoaded {
			t, err := targetTournament(s.db)
			if err != nil {
				log.Printf("telemetria: %v", err)
			}
			s.target, s.targetLoaded = t, err == nil
		}
		v.Tournament = s.target
	}

	parts, class := s.tracker.Participants(), s.tracker.Classification()
	for _, c := range l.cars {
		car := LineupCar{
			Controller: c.controller, CarIndex: c.carIndex,
			PlayerID: c.playerID, PlayerName: c.playerName, PlayerColor: c.playerColor,
		}
		if parts != nil && c.carIndex < len(parts.Cars) {
			p := parts.Cars[c.carIndex]
			car.Name, car.Team, car.RaceNumber = p.Name, p.TeamID.String(), p.RaceNumber
		}
		if class != nil && c.carIndex < len(class.Cars) {
			r := class.Cars[c.carIndex]
			st := int(r.ResultStatus)
			car.DNF = !classified(&st)
			if pos := int(r.Position); pos > 0 && !car.DNF {
				car.Position = &pos
			}
		}
		v.Cars = append(v.Cars, car)
	}
	return v
}

// rememberSaved conserva il riepilogo della sessione cur, che sta per finire,
// se è stata salvata nel torneo. Il tracker ha ancora i suoi dati.
func (s *Service) rememberSaved(cur *session, at time.Time) {
	if cur.lineup == nil || cur.lineup.stage != StageSaved {
		return
	}
	ses := s.tracker.Session()
	s.lastSaved = &SavedLineup{
		Lineup: *s.lineupOf(cur, at), Track: ses.TrackID.String(), Session: ses.SessionType.String(),
		until: at.Add(lastSavedFor),
	}
}

// savedView restituisce il riepilogo dell'ultima sessione salvata, se è ancora da mostrare.
func (s *Service) savedView(now time.Time) *SavedLineup {
	if s.lastSaved == nil || now.After(s.lastSaved.until) {
		return nil
	}
	return s.lastSaved
}

// updateSaved aggiorna i piloti nel riepilogo dell'ultima sessione salvata,
// dopo una correzione con SetSessionPlayers.
func (s *Service) updateSaved(sessionID int64, players map[int]*int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.lastSaved
	if v == nil || v.SessionID != sessionID {
		return
	}
	cars := slices.Clone(v.Cars)
	for i, c := range cars {
		id, ok := players[c.CarIndex]
		if !ok {
			continue
		}
		var p scoreboardPlayer
		if id != nil {
			var err error
			if p, err = playerInfo(s.db, *id); err != nil {
				log.Printf("telemetria: %v", err)
				continue
			}
		}
		cars[i].PlayerID, cars[i].PlayerName, cars[i].PlayerColor = id, p.Name, p.Color
	}
	v.Cars = cars
}

// LineupChoice è il pilota scelto per un'auto.
type LineupChoice struct {
	CarIndex int    `json:"carIndex"`
	PlayerID *int64 `json:"playerId"`
}

// SetLineup cambia i piloti alla guida delle auto dei giocatori nella sessione
// sessionID. Una modifica ferma il salvataggio automatico; confirm chiude la
// fase attuale: la scelta prima della partenza, oppure i risultati, che vengono
// salvati nel torneo. Restituisce il torneo in cui è salvata la sessione, nil
// se non lo è ancora.
//
// La sessione può essere finita (e salvata da sola) mentre la pagina ne
// mostrava ancora i risultati: la conferma allora non tocca la sessione
// successiva, ma dice dove è stata salvata.
func (s *Service) SetLineup(sessionID int64, choices []LineupChoice, confirm bool) (*LineupTournament, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cur := s.current
	if cur == nil || cur.lineup == nil || cur.id != sessionID {
		if v := s.lastSaved; v != nil && v.SessionID == sessionID {
			if len(choices) == 0 {
				return v.Tournament, nil
			}
			return nil, invalid("La sessione è terminata ed è già stata salvata: correggi i piloti nel riepilogo")
		}
		return nil, ErrNoLineup
	}
	l := cur.lineup
	if l.stage == StageSaved {
		if len(choices) == 0 {
			return l.saved, nil
		}
		return nil, invalid("I risultati sono già stati salvati: per correggerli usa la pagina delle registrazioni")
	}

	cars := slices.Clone(l.cars)
	changed := false
	for _, ch := range choices {
		i := slices.IndexFunc(cars, func(c lineupCar) bool { return c.carIndex == ch.CarIndex })
		if i < 0 {
			return nil, invalid("Nessun giocatore sull'auto %d", ch.CarIndex)
		}
		var p scoreboardPlayer
		if ch.PlayerID != nil {
			var err error
			p, err = playerInfo(s.db, *ch.PlayerID)
			if errors.Is(err, sql.ErrNoRows) {
				return nil, invalid("Pilota %d inesistente", *ch.PlayerID)
			}
			if err != nil {
				return nil, err
			}
		}
		if !samePlayer(cars[i].playerID, ch.PlayerID) {
			changed = true
		}
		cars[i].playerID, cars[i].playerName, cars[i].playerColor = ch.PlayerID, p.Name, p.Color
	}
	for i, a := range cars {
		for _, b := range cars[i+1:] {
			if a.playerID != nil && samePlayer(a.playerID, b.playerID) {
				return nil, invalid("%s non può guidare due auto", a.playerName)
			}
		}
	}

	l.cars = cars
	if changed {
		l.deadline = time.Time{} // l'utente è intervenuto: si aspetta la sua conferma
	}
	if confirm {
		switch l.stage {
		case StageDrivers:
			l.stage, l.deadline = StageRacing, time.Time{}
		case StageResults:
			if err := s.saveLineup(cur, time.Now()); err != nil {
				return nil, err
			}
		}
	}
	return l.saved, nil
}

func samePlayer(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
