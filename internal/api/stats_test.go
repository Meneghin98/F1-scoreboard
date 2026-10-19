package api_test

import (
	"database/sql"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	"f1-scoreboard/internal/api"
	"f1-scoreboard/internal/live"
	"f1-scoreboard/internal/store"
)

// newTestServerDB è newTestServer con accesso al database, per preparare dati
// che le API non scrivono (come quelli delle registrazioni).
func newTestServerDB(t *testing.T) (*httptest.Server, *sql.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(api.New(db, fstest.MapFS{}, live.NewService(db, live.Options{Dir: t.TempDir()})))
	t.Cleanup(func() { db.Close() })
	t.Cleanup(srv.Close)
	return srv, db
}

func TestStats(t *testing.T) {
	srv, db := newTestServerDB(t)
	var cup struct {
		ID int `json:"id"`
	}
	call(t, srv, "POST", "/tournaments", map[string]string{"name": "Coppa"}, &cup)
	for _, name := range []string{"Simone", "Luca"} {
		call(t, srv, "POST", "/players", map[string]string{"name": name}, nil)
	}
	race := func(track, raceType string) int {
		var r struct {
			ID int `json:"id"`
		}
		call(t, srv, "POST", fmt.Sprintf("/tournaments/%d/races", cup.ID), map[string]string{"track": track, "type": raceType}, &r)
		return r.ID
	}
	type res struct {
		PlayerID   int  `json:"player_id"`
		Position   *int `json:"position,omitempty"`
		DNF        bool `json:"dnf,omitempty"`
		FastestLap bool `json:"fastest_lap,omitempty"`
	}
	save := func(raceID int, results ...res) int {
		return call(t, srv, "PUT", fmt.Sprintf("/races/%d/results", raceID), map[string]any{"results": results}, nil)
	}
	quali, monza, imola := race("Italia – Monza", "qualifying"), race("Italia – Monza", "race"), race("Emilia-Romagna – Imola", "race")
	save(quali, res{PlayerID: 1, Position: pos(1)}, res{PlayerID: 2, Position: pos(2)})
	expectStatus(t, save(monza, res{PlayerID: 1, Position: pos(1), FastestLap: true}, res{PlayerID: 2, Position: pos(2), FastestLap: true}),
		400, "due giri veloci")
	save(monza, res{PlayerID: 1, Position: pos(1), FastestLap: true}, res{PlayerID: 2, Position: pos(2)})
	save(imola, res{PlayerID: 1, DNF: true}, res{PlayerID: 2, Position: pos(3)})

	var standings struct {
		Standings []struct {
			Name        string `json:"name"`
			Poles       int    `json:"poles"`
			FastestLaps int    `json:"fastestLaps"`
		} `json:"standings"`
	}
	call(t, srv, "GET", fmt.Sprintf("/tournaments/%d/standings", cup.ID), nil, &standings)
	// Luca è primo (18 + 15 punti), Simone secondo (25) ma con pole e giro veloce.
	if s := standings.Standings[1]; s.Name != "Simone" || s.Poles != 1 || s.FastestLaps != 1 {
		t.Fatalf("classifica %+v", standings.Standings)
	}

	// Un giro registrato dalla telemetria: diventa il record di Monza.
	var session int64
	db.QueryRow(`INSERT INTO telemetry_sessions (session_uid, packet_format, started_at, file, track_id, session_type, race_id)
		VALUES ('1', 2025, '2026-10-18 20:00:00', 'x.f1rec.gz', 11, 15, ?) RETURNING id`, monza).Scan(&session)
	db.Exec(`INSERT INTO telemetry_players (session_id, car_index, name, team, race_number, player_id, best_lap_ms)
		VALUES (?, 0, 'HAMILTON', 'Ferrari', 44, 2, 81234), (?, 1, 'LECLERC', 'Ferrari', 16, 1, 81999)`, session, session)

	var stats struct {
		Career []struct {
			Name        string   `json:"name"`
			Races       int      `json:"races"`
			Wins        int      `json:"wins"`
			Podiums     int      `json:"podiums"`
			Poles       int      `json:"poles"`
			FastestLaps int      `json:"fastestLaps"`
			DNF         int      `json:"dnf"`
			AvgFinish   *float64 `json:"avgFinish"`
			Points      int      `json:"points"`
		} `json:"career"`
		Records []struct {
			Track string `json:"track"`
			LapMS int    `json:"lapMs"`
			Name  string `json:"name"`
		} `json:"records"`
		HeadToHead struct {
			Players []struct {
				Name string `json:"name"`
			} `json:"players"`
			Ahead [][]int `json:"ahead"`
		} `json:"headToHead"`
	}
	expectStatus(t, call(t, srv, "GET", "/stats", nil, &stats), 200, "statistiche")
	if len(stats.Career) != 2 {
		t.Fatalf("carriera %+v", stats.Career)
	}
	luca, simone := stats.Career[0], stats.Career[1]
	if simone.Name != "Simone" || simone.Races != 2 || simone.Wins != 1 || simone.Podiums != 1 || simone.Poles != 1 ||
		simone.FastestLaps != 1 || simone.DNF != 1 || simone.Points != 25 || *simone.AvgFinish != 1 {
		t.Errorf("Simone %+v", simone)
	}
	if luca.Points != 18+15 || luca.Podiums != 2 || *luca.AvgFinish != 2.5 {
		t.Errorf("Luca %+v", luca)
	}
	if len(stats.Records) != 1 || stats.Records[0].Track != "Italia – Monza" || stats.Records[0].LapMS != 81234 || stats.Records[0].Name != "Luca" {
		t.Errorf("record %+v", stats.Records)
	}
	// Monza: Simone davanti; Imola: Simone ritirato, Luca davanti.
	if h := stats.HeadToHead; h.Players[0].Name != "Luca" || h.Ahead[0][1] != 1 || h.Ahead[1][0] != 1 {
		t.Errorf("testa a testa %+v", h)
	}

	var profile struct {
		Career struct {
			Wins int `json:"wins"`
		} `json:"career"`
		Tournaments []struct {
			Name string `json:"name"`
			Rank int    `json:"rank"`
		} `json:"tournaments"`
		BestLaps []struct {
			LapMS  int  `json:"lapMs"`
			Record bool `json:"record"`
		} `json:"bestLaps"`
		Recent []struct {
			Track string `json:"track"`
		} `json:"recent"`
		Rivals []struct {
			Name   string `json:"name"`
			Ahead  int    `json:"ahead"`
			Behind int    `json:"behind"`
		} `json:"rivals"`
	}
	expectStatus(t, call(t, srv, "GET", "/players/1/stats", nil, &profile), 200, "profilo")
	if profile.Career.Wins != 1 || len(profile.Tournaments) != 1 || profile.Tournaments[0].Rank != 2 ||
		len(profile.BestLaps) != 1 || profile.BestLaps[0].LapMS != 81999 || profile.BestLaps[0].Record ||
		len(profile.Recent) != 3 || len(profile.Rivals) != 1 || profile.Rivals[0].Ahead != 1 || profile.Rivals[0].Behind != 1 {
		t.Errorf("profilo %+v", profile)
	}
	expectStatus(t, call(t, srv, "GET", "/players/99/stats", nil, nil), 404, "pilota inesistente")
}
