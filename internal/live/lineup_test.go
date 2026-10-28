package live

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"f1-scoreboard/internal/recording"
	"f1-scoreboard/internal/telemetry"
)

// raceOnly restituisce i pacchetti della gara (l'ultima sessione della
// registrazione), con il SessionUID cambiato in uid: così la stessa gara si
// può far correre più volte come se fossero sessioni diverse.
func raceOnly(t *testing.T, packets []recording.Packet, uid uint64) []recording.Packet {
	t.Helper()
	var last uint64
	for _, p := range packets {
		if h, err := telemetry.ParseHeader(p.Data); err == nil && h.SessionUID != 0 {
			last = h.SessionUID
		}
	}
	var out []recording.Packet
	for _, p := range packets {
		if h, err := telemetry.ParseHeader(p.Data); err == nil && h.SessionUID == last {
			data := append([]byte(nil), p.Data...)
			binary.LittleEndian.PutUint64(data[7:], uid)
			out = append(out, recording.Packet{At: p.At, Data: data})
		}
	}
	return out
}

// liveSnapshot restituisce lo stato all'istante dell'ultimo pacchetto: i
// pacchetti dei test hanno gli orari della registrazione, non quelli di adesso.
func liveSnapshot(t *testing.T, s *Service) Snapshot {
	t.Helper()
	s.mu.Lock()
	msg := s.snapshotJSON(s.lastPacket)
	s.mu.Unlock()
	var snap Snapshot
	if err := json.Unmarshal(msg, &snap); err != nil {
		t.Fatal(err)
	}
	return snap
}

// Senza nessun intervento la gara finisce in "Nessun torneo".
func TestLineupSavesWithoutTournament(t *testing.T) {
	s, _ := recordedService(t)
	records, err := s.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	r := records[0]
	if r.Race == nil || r.Race.TournamentName != "Nessun torneo" || r.Race.Type != "race" || r.Race.Track != "Italia – Monza" {
		t.Fatalf("gara %+v", r.Race)
	}
	// Le sessioni che non sono gare non finiscono in nessun torneo.
	for _, other := range records[1:] {
		if other.Race != nil {
			t.Errorf("sessione %d (%s) collegata a %+v", other.ID, other.Type, other.Race)
		}
	}
}

func TestLineupFlow(t *testing.T) {
	packets := loadRecording(t)
	s, _ := newTestService(t, false)
	marco := insertID(t, s, "INSERT INTO players (name) VALUES ('Marco')")
	luca := insertID(t, s, "INSERT INTO players (name) VALUES ('Luca')")
	// Senza turnazione: i piloti non li propone lei (vedi TestLineupFollowsRotation).
	cup := insertID(t, s, "INSERT INTO tournaments (name, started_at, rotation) VALUES ('Campionato', datetime('now'), 0)")

	base := time.Date(2026, 10, 16, 13, 31, 38, 0, time.UTC)
	stages := map[string]bool{}
	var carIndex int
	for _, p := range raceOnly(t, packets, 1) {
		s.Handle(base.Add(p.At), p.Data)
		l := s.current.lineup
		if l == nil || stages[l.stage] {
			continue
		}
		stages[l.stage] = true
		snap := liveSnapshot(t, s)
		v := snap.Lineup
		switch l.stage {
		case StageDrivers:
			if v == nil || len(v.Cars) != 1 || v.Cars[0].Controller != 1 || v.Cars[0].Name != "HAMILTON" ||
				v.Cars[0].PlayerID != nil || v.SecondsLeft == nil || *v.SecondsLeft != 300 ||
				v.Tournament == nil || v.Tournament.Name != "Campionato" {
				t.Fatalf("scelta dei piloti %+v", v)
			}
			carIndex = v.Cars[0].CarIndex
			// Una modifica ferma il conto alla rovescia.
			if _, err := s.SetLineup(s.current.id, []LineupChoice{{CarIndex: carIndex, PlayerID: &marco}}, false); err != nil {
				t.Fatal(err)
			}
			if v := liveSnapshot(t, s).Lineup; v.SecondsLeft != nil || v.Cars[0].PlayerName != "Marco" {
				t.Fatalf("dopo la modifica %+v", v)
			}
		case StageResults:
			if v.Cars[0].Position == nil || *v.Cars[0].Position != 7 || v.SecondsLeft == nil {
				t.Fatalf("risultati %+v", v)
			}
			for _, d := range snap.Drivers {
				if d.CarIndex == carIndex && d.PlayerName != "Marco" {
					t.Errorf("nome in classifica %q", d.PlayerName)
				}
			}
			// Correzione e conferma: la gara viene salvata subito nel torneo in corso.
			if saved, err := s.SetLineup(s.current.id, []LineupChoice{{CarIndex: carIndex, PlayerID: &luca}}, true); err != nil || saved == nil || saved.ID != cup {
				t.Fatalf("conferma: %v, %+v", err, saved)
			}
			v = liveSnapshot(t, s).Lineup
			if v.Stage != StageSaved || v.Tournament.ID != cup || v.RaceID == nil {
				t.Fatalf("dopo la conferma %+v", v)
			}
			if res := resultsOf(t, s, *v.RaceID); len(res) != 1 || res[0].playerID != luca || *res[0].position != 7 {
				t.Fatalf("risultati %+v", res)
			}
			if _, err := s.SetLineup(s.current.id, []LineupChoice{{CarIndex: carIndex, PlayerID: &marco}}, false); err == nil {
				t.Error("modifica dopo il salvataggio accettata")
			}
			// Confermare di nuovo dice solo dove è stata salvata.
			if saved, err := s.SetLineup(s.current.id, nil, true); err != nil || saved == nil || saved.ID != cup {
				t.Errorf("seconda conferma: %v, %+v", err, saved)
			}
		}
	}
	if !stages[StageDrivers] || !stages[StageRacing] || !stages[StageResults] || !stages[StageSaved] {
		t.Fatalf("fasi viste: %v", stages)
	}

	// A fine sessione i piloti confermati non vengono sovrascritti.
	s.tick(s.lastPacket.Add(sessionTimeout + time.Second))
	records, _ := s.Sessions()
	if r := records[0]; r.Race == nil || r.Race.TournamentID != cup || *r.Players[0].PlayerID != luca {
		t.Fatalf("sessione %+v", r)
	}

	// Alla gara successiva lo stesso controller viene proposto con lo stesso pilota,
	// e senza conferme i risultati si salvano allo scadere del tempo.
	race := raceOnly(t, packets, 2)
	for _, p := range race {
		at := base.Add(time.Hour + p.At)
		s.Handle(at, p.Data)
		l := s.current.lineup
		if l != nil && l.stage == StageDrivers && (l.cars[0].playerID == nil || *l.cars[0].playerID != luca) {
			t.Fatalf("pilota proposto %+v", l.cars[0])
		}
		if l != nil && l.stage == StageResults {
			s.updateLineup(at.Add(lineupTimeout))
			break
		}
	}
	if v := liveSnapshot(t, s).Lineup; v.Stage != StageSaved || v.Tournament.ID != cup {
		t.Fatalf("salvataggio automatico %+v", v)
	}
}

// Se nel gioco si passa alla sessione successiva mentre la pagina mostra ancora
// i risultati, la gara si salva da sola: la conferma arrivata in ritardo non
// tocca la sessione nuova, e il riepilogo resta visibile e correggibile.
func TestLineupConfirmAfterSessionEnd(t *testing.T) {
	packets := loadRecording(t)
	s, _ := newTestService(t, false)
	luca := insertID(t, s, "INSERT INTO players (name) VALUES ('Luca')")
	cup := insertID(t, s, "INSERT INTO tournaments (name, started_at) VALUES ('Campionato', datetime('now'))")

	base := time.Date(2026, 10, 16, 13, 31, 38, 0, time.UTC)
	var first int64
	var carIndex int
	for _, p := range raceOnly(t, packets, 1) {
		s.Handle(base.Add(p.At), p.Data)
		if l := s.current.lineup; l != nil && l.stage == StageResults {
			first, carIndex = s.current.id, l.cars[0].carIndex
			break
		}
	}
	if first == 0 {
		t.Fatal("risultati mai arrivati")
	}

	// Parte la sessione successiva, ancora alla scelta dei piloti.
	next := raceOnly(t, packets, 2)
	rest := next
	for i, p := range next {
		s.Handle(base.Add(time.Hour+p.At), p.Data)
		if l := s.current.lineup; l != nil {
			rest = next[i+1:]
			break
		}
	}
	if l := s.current.lineup; l == nil || l.stage != StageDrivers {
		t.Fatalf("sessione successiva %+v", l)
	}
	v := liveSnapshot(t, s).LastSaved
	if v == nil || v.SessionID != first || v.Stage != StageSaved || v.Tournament == nil || v.Tournament.ID != cup ||
		v.Track != "Monza" || v.Cars[0].Position == nil || *v.Cars[0].Position != 7 {
		t.Fatalf("riepilogo %+v", v)
	}

	if saved, err := s.SetLineup(first, nil, true); err != nil || saved == nil || saved.ID != cup {
		t.Fatalf("conferma in ritardo: %v, %+v", err, saved)
	}
	if l := s.current.lineup; l.stage != StageDrivers {
		t.Fatalf("la conferma ha chiuso la scelta della sessione successiva: %s", l.stage)
	}
	var invalidErr *InvalidError
	if _, err := s.SetLineup(first, []LineupChoice{{CarIndex: carIndex, PlayerID: &luca}}, false); !errors.As(err, &invalidErr) {
		t.Errorf("modifica in ritardo: %v", err)
	}

	// La correzione dalla scheda del riepilogo si vede subito.
	if err := s.SetSessionPlayers(first, map[int]*int64{carIndex: &luca}); err != nil {
		t.Fatal(err)
	}
	if v := liveSnapshot(t, s).LastSaved; v == nil || v.Cars[0].PlayerName != "Luca" {
		t.Fatalf("dopo la correzione %+v", v)
	}
	if s.savedView(s.lastSaved.until.Add(time.Second)) != nil {
		t.Error("riepilogo ancora visibile dopo la scadenza")
	}

	// Alla partenza della sessione successiva il riepilogo sparisce.
	for _, p := range rest {
		s.Handle(base.Add(time.Hour+p.At), p.Data)
		if s.current.lineup.stage != StageDrivers {
			break
		}
	}
	if v := liveSnapshot(t, s).LastSaved; v != nil {
		t.Fatalf("riepilogo dopo la partenza %+v", v)
	}
}

func TestSetLineupErrors(t *testing.T) {
	s, _ := newTestService(t, false)
	if _, err := s.SetLineup(0, nil, true); err != ErrNoLineup {
		t.Fatalf("senza gara: %v", err)
	}

	marco := insertID(t, s, "INSERT INTO players (name) VALUES ('Marco')")
	s.current = &session{lineup: &lineup{stage: StageDrivers, cars: []lineupCar{{controller: 1, carIndex: 3}, {controller: 2, carIndex: 5}}}}
	t.Cleanup(func() { s.current = nil })
	missing := int64(99)
	for name, choices := range map[string][]LineupChoice{
		"auto inesistente":   {{CarIndex: 7, PlayerID: &marco}},
		"pilota inesistente": {{CarIndex: 3, PlayerID: &missing}},
		"pilota su due auto": {{CarIndex: 3, PlayerID: &marco}, {CarIndex: 5, PlayerID: &marco}},
	} {
		var invalidErr *InvalidError
		if _, err := s.SetLineup(0, choices, false); !errors.As(err, &invalidErr) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Uno scambio tra le due auto invece va bene.
	if _, err := s.SetLineup(0, []LineupChoice{{CarIndex: 3, PlayerID: &marco}}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetLineup(0, []LineupChoice{{CarIndex: 3, PlayerID: nil}, {CarIndex: 5, PlayerID: &marco}}, true); err != nil {
		t.Fatal(err)
	}
	if l := s.current.lineup; l.stage != StageRacing || l.cars[1].playerName != "Marco" {
		t.Fatalf("dopo lo scambio %+v", l)
	}
}

func TestIsSprint(t *testing.T) {
	sprintWeekend := []telemetry.SessionType{
		telemetry.SessionPractice1, telemetry.SessionSprintShootout1, telemetry.SessionRace,
		telemetry.SessionQualifying1, telemetry.SessionRace2,
	}
	normal := []telemetry.SessionType{telemetry.SessionPractice1, telemetry.SessionQualifying1, telemetry.SessionRace}
	cases := []struct {
		weekend []telemetry.SessionType
		session telemetry.SessionType
		want    bool
	}{
		{sprintWeekend, telemetry.SessionRace, true},
		{sprintWeekend, telemetry.SessionRace2, false},
		{normal, telemetry.SessionRace, false},
		{nil, telemetry.SessionRace, false},
	}
	for _, c := range cases {
		ses := &telemetry.Session{SessionType: c.session, WeekendStructure: c.weekend}
		if got := isSprint(ses); got != c.want {
			t.Errorf("weekend %v, sessione %v: %v", c.weekend, c.session, got)
		}
	}
}

// Le sessioni finite registrate prima della migrazione 006 finiscono in "Nessun torneo", una volta sola.
func TestAdoptPending(t *testing.T) {
	s, _ := recordedService(t)
	records, _ := s.Sessions()
	last := records[0]
	marco := insertID(t, s, "INSERT INTO players (name) VALUES ('Marco')")
	if err := s.SetSessionPlayers(last.ID, map[int]*int64{last.Players[0].CarIndex: &marco}); err != nil {
		t.Fatal(err)
	}
	// Come prima della migrazione: sessione senza gara, segnata dalla migrazione.
	s.exec("DELETE FROM races")
	s.exec("UPDATE telemetry_sessions SET needs_tournament = 1 WHERE id = ?", last.ID)

	s.adoptPending()
	records, _ = s.Sessions()
	r := records[0]
	if r.Race == nil || r.Race.TournamentName != "Nessun torneo" || r.Race.Type != "race" {
		t.Fatalf("gara %+v", r.Race)
	}
	if res := resultsOf(t, s, r.Race.ID); len(res) != 1 || res[0].playerID != marco || *res[0].position != 7 {
		t.Fatalf("risultati %+v", res)
	}

	// Eliminata la gara a mano, non ricompare al riavvio successivo.
	s.exec("DELETE FROM races")
	s.adoptPending()
	if records, _ = s.Sessions(); records[0].Race != nil {
		t.Fatalf("gara ricomparsa: %+v", records[0].Race)
	}
}

// sessionKind e sessionKindSQL devono dare lo stesso risultato.
func TestSessionKindSQL(t *testing.T) {
	s, _ := newTestService(t, false)
	for st := range 20 {
		var kind *string
		if err := s.db.QueryRow("SELECT "+sessionKindSQL+" FROM (SELECT ? AS session_type) s", st).Scan(&kind); err != nil {
			t.Fatal(err)
		}
		got := ""
		if kind != nil {
			got = *kind
		}
		if want := sessionKind(telemetry.SessionType(st)); got != want {
			t.Errorf("tipo %d: SQL %q, Go %q", st, got, want)
		}
	}
}
