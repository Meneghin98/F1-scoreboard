package live

import (
	"fmt"
	"testing"
	"time"

	"f1-scoreboard/internal/rotation"
)

// Con la turnazione attiva il controller va al pilota che tocca a lui, e
// quando un Gran Premio entra nel torneo si passa alla coppia successiva.
func TestLineupFollowsRotation(t *testing.T) {
	packets := loadRecording(t)
	s, _ := newTestService(t, false)
	marco := insertID(t, s, "INSERT INTO players (name) VALUES ('Marco')")
	luca := insertID(t, s, "INSERT INTO players (name) VALUES ('Luca')")
	anna := insertID(t, s, "INSERT INTO players (name) VALUES ('Anna')")
	cup := insertID(t, s, "INSERT INTO tournaments (name, started_at, rotation_order) VALUES ('Campionato', datetime('now'), ?)",
		fmt.Sprintf("%d,%d,%d", luca, marco, anna))
	if err := rotation.SetPresent(s.db, []int64{marco, luca, anna}); err != nil {
		t.Fatal(err)
	}
	s.DataChanged()

	if r := liveSnapshot(t, s).Rotation; r == nil || r.Tournament == nil || r.Tournament.ID != cup ||
		len(r.Next) != rotation.Preview || r.Next[0] != (rotation.Pair{P1: luca, P2: marco}) {
		t.Fatalf("turnazione prima della gara %+v", r)
	}

	// proposed fa correre la gara fino alla scelta dei piloti e restituisce chi
	// è proposto per il controller 1 (la registrazione ha un solo giocatore).
	proposed := func(uid uint64, base time.Time) int64 {
		t.Helper()
		for _, p := range raceOnly(t, packets, uid) {
			s.Handle(base.Add(p.At), p.Data)
			if l := s.current.lineup; l != nil && l.stage == StageDrivers {
				if l.cars[0].playerID == nil {
					t.Fatalf("nessun pilota proposto %+v", l.cars[0])
				}
				return *l.cars[0].playerID
			}
		}
		t.Fatal("la scelta dei piloti non è mai iniziata")
		return 0
	}
	base := time.Date(2026, 10, 16, 13, 31, 38, 0, time.UTC)
	if got := proposed(1, base); got != luca {
		t.Fatalf("controller 1 proposto a %d, atteso Luca (%d)", got, luca)
	}
	s.tick(s.lastPacket.Add(sessionTimeout + time.Second))

	// Un Gran Premio di Luca e Marco inserito a mano: ora tocca ad Anna. Senza
	// registrazione il controller 1 vale per Marco (id più basso), quindi lo
	// riprende Luca, primo nel sorteggio.
	race := insertID(t, s, "INSERT INTO races (tournament_id, track, type, date) VALUES (?, 'Monza', 'race', '2026-10-01')", cup)
	if _, err := s.db.Exec("INSERT INTO results (race_id, player_id, position) VALUES (?, ?, 1), (?, ?, 2)", race, luca, race, marco); err != nil {
		t.Fatal(err)
	}
	s.DataChanged()
	if r := liveSnapshot(t, s).Rotation; r.Next[0] != (rotation.Pair{P1: luca, P2: anna}) {
		t.Fatalf("turnazione dopo il GP %+v", r.Next)
	}
	if got := proposed(2, base.Add(time.Hour)); got != luca {
		t.Fatalf("controller 1 proposto a %d, atteso Luca (%d)", got, luca)
	}
}

// Con la turnazione spenta nel torneo i controller non li propone lei, e tra
// una sessione e l'altra non ci sono coppie da mostrare.
func TestLineupWithoutRotation(t *testing.T) {
	s, _ := newTestService(t, false)
	insertID(t, s, "INSERT INTO players (name) VALUES ('Marco')")
	insertID(t, s, "INSERT INTO players (name) VALUES ('Luca')")
	cup := insertID(t, s, "INSERT INTO tournaments (name, started_at, rotation) VALUES ('Campionato', datetime('now'), 0)")
	s.DataChanged()

	if r := liveSnapshot(t, s).Rotation; r == nil || r.Tournament == nil || r.Tournament.ID != cup || r.Enabled || len(r.Next) != 0 {
		t.Fatalf("turnazione spenta %+v", r)
	}
	if p := s.rotationPlayers(); p != nil {
		t.Fatalf("piloti proposti dalla turnazione spenta: %v", p)
	}
}
