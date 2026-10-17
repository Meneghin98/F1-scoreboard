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

const lineupTimeout = 5 * time.Minute

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
}

type lineupCar struct {
	controller int // 1 o 2
	carIndex   int
	playerID   *int64
	playerName string
}

// Lineup è la scelta dei piloti come arriva al frontend.
type Lineup struct {
	Stage string `json:"stage"`
	Kind  string `json:"kind"` // "race", "qualifying" o "practice", vedi sessionKind
	// SecondsLeft sono i secondi al salvataggio automatico, nil se non è previsto.
	SecondsLeft *int        `json:"secondsLeft"`
	Cars        []LineupCar `json:"cars"`
	// Tournament è il torneo in cui finirà la gara (o in cui è finita, se salvata).
	Tournament *LineupTournament `json:"tournament"`
	RaceID     *int64            `json:"raceId"` // gara creata nel torneo, dopo il salvataggio
}

type LineupCar struct {
	Controller int    `json:"controller"`
	CarIndex   int    `json:"carIndex"`
	Name       string `json:"name"` // nome mostrato dal gioco
	Team       string `json:"team"`
	RaceNumber uint8  `json:"raceNumber"`
	PlayerID   *int64 `json:"playerId"`
	PlayerName string `json:"playerName"`
	Position   *int   `json:"position"` // dalla classifica finale
	DNF        bool   `json:"dnf"`
}

type LineupTournament struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Free bool   `json:"free"`
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
	if l.stage == StageRacing && s.tracker.Classification() != nil {
		l.stage, l.deadline = StageResults, at.Add(lineupTimeout)
	}
	if l.stage == StageResults && expired(l, at) {
		if err := s.saveLineup(cur); err != nil {
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
			car.playerID, car.playerName = id, s.playerName(*id)
		}
		l.cars = append(l.cars, car)
	}
	cur.lineup = l
}

func (s *Service) playerName(id int64) string {
	var name string
	if err := s.db.QueryRow("SELECT name FROM players WHERE id = ?", id).Scan(&name); err != nil {
		log.Printf("telemetria: %v", err)
	}
	return name
}

func (l *lineup) assigned() map[int]*int64 {
	m := make(map[int]*int64, len(l.cars))
	for _, c := range l.cars {
		m[c.carIndex] = c.playerID
	}
	return m
}

// saveLineup salva i giocatori della sessione con i piloti scelti e, se la
// gara ha la classifica finale, la aggiunge al torneo in corso (o in "Nessun torneo").
func (s *Service) saveLineup(cur *session) error {
	l := cur.lineup
	l.stage, l.deadline = StageSaved, time.Time{}
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
	t, raceID, err := s.addRaceToTournament(cur.id, ses)
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

// addRaceToTournament crea la gara nel torneo in corso (o in "Nessun torneo") e
// le collega la sessione. Se la sessione è già collegata a una gara (a mano,
// dalla pagina delle registrazioni) resta dov'è.
func (s *Service) addRaceToTournament(sessionID int64, ses *telemetry.Session) (*LineupTournament, *int64, error) {
	var raceID *int64
	if err := s.db.QueryRow("SELECT race_id FROM telemetry_sessions WHERE id = ?", sessionID).Scan(&raceID); err != nil {
		return nil, nil, err
	}
	if raceID != nil {
		var t LineupTournament
		err := s.db.QueryRow(`
			SELECT t.id, t.name, t.free FROM races r JOIN tournaments t ON t.id = r.tournament_id WHERE r.id = ?`, *raceID).
			Scan(&t.ID, &t.Name, &t.Free)
		return &t, raceID, err
	}

	t, err := targetTournament(s.db)
	if err != nil || t == nil {
		return nil, nil, err
	}
	raceType := raceTypeFor(ses.SessionType)
	if isSprint(ses) {
		raceType = "sprint"
	}
	id, err := s.LinkNewRace(sessionID, t.ID, raceType)
	if err != nil {
		return nil, nil, err
	}
	return t, &id, nil
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

// TournamentsChanged va chiamato quando cambia il torneo in corso (o un nome):
// il torneo mostrato nella pagina Live viene riletto.
func (s *Service) TournamentsChanged() {
	s.mu.Lock()
	s.target, s.targetLoaded = nil, false
	s.mu.Unlock()
}

// lineupView prepara la scelta dei piloti per il frontend e scrive i nomi dei
// piloti scelti nella classifica live.
func (s *Service) lineupView(snap *Snapshot, now time.Time) {
	cur := s.current
	if cur == nil || cur.lineup == nil {
		return
	}
	l := cur.lineup
	v := &Lineup{Stage: l.stage, Kind: sessionKind(s.tracker.Session().SessionType), Cars: []LineupCar{}, Tournament: l.saved, RaceID: l.raceID}
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
		car := LineupCar{Controller: c.controller, CarIndex: c.carIndex, PlayerID: c.playerID, PlayerName: c.playerName}
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
		for i := range snap.Drivers {
			if snap.Drivers[i].CarIndex == c.carIndex {
				snap.Drivers[i].PlayerName = c.playerName
			}
		}
	}
	snap.Lineup = v
}

// LineupChoice è il pilota scelto per un'auto.
type LineupChoice struct {
	CarIndex int    `json:"carIndex"`
	PlayerID *int64 `json:"playerId"`
}

// SetLineup cambia i piloti alla guida delle auto dei giocatori. Una modifica
// ferma il salvataggio automatico; confirm chiude la fase attuale: la scelta
// prima della partenza, oppure i risultati, che vengono salvati nel torneo.
func (s *Service) SetLineup(choices []LineupChoice, confirm bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cur := s.current
	if cur == nil || cur.lineup == nil {
		return ErrNoLineup
	}
	l := cur.lineup
	if l.stage == StageSaved {
		return invalid("I risultati sono già stati salvati: per correggerli usa la pagina delle registrazioni")
	}

	cars := slices.Clone(l.cars)
	changed := false
	for _, ch := range choices {
		i := slices.IndexFunc(cars, func(c lineupCar) bool { return c.carIndex == ch.CarIndex })
		if i < 0 {
			return invalid("Nessun giocatore sull'auto %d", ch.CarIndex)
		}
		var name string
		if ch.PlayerID != nil {
			err := s.db.QueryRow("SELECT name FROM players WHERE id = ?", *ch.PlayerID).Scan(&name)
			if errors.Is(err, sql.ErrNoRows) {
				return invalid("Pilota %d inesistente", *ch.PlayerID)
			}
			if err != nil {
				return err
			}
		}
		if !samePlayer(cars[i].playerID, ch.PlayerID) {
			changed = true
		}
		cars[i].playerID, cars[i].playerName = ch.PlayerID, name
	}
	for i, a := range cars {
		for _, b := range cars[i+1:] {
			if a.playerID != nil && samePlayer(a.playerID, b.playerID) {
				return invalid("%s non può guidare due auto", a.playerName)
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
			return s.saveLineup(cur)
		}
	}
	return nil
}

func samePlayer(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
