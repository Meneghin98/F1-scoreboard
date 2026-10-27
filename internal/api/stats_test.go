package api_test

import (
	"fmt"
	"testing"
)

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

	// Giri registrati dalla telemetria a Monza: in gara il record è di Luca, in qualifica
	// di Simone; il giro più veloce, in prova, non è un record.
	lap := func(sessionType int, raceID any, laps ...any) {
		var session int64
		db.QueryRow(`INSERT INTO telemetry_sessions (session_uid, packet_format, started_at, file, track_id, session_type, race_id)
			VALUES (?, 2025, '2026-10-18 20:00:00', 'x.f1rec.gz', 11, ?, ?) RETURNING id`, sessionType, sessionType, raceID).Scan(&session)
		for i := 0; i < len(laps); i += 2 {
			db.Exec(`INSERT INTO telemetry_players (session_id, car_index, name, team, race_number, player_id, best_lap_ms)
				VALUES (?, ?, 'PILOTA', 'Ferrari', 1, ?, ?)`, session, i, laps[i], laps[i+1])
		}
	}
	lap(15, monza, 2, 81234, 1, 81999)
	lap(7, quali, 1, 80100, 2, 80500)
	lap(1, nil, 2, 79000)

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
			Track      string     `json:"track"`
			Race       *lapRecord `json:"race"`
			Qualifying *lapRecord `json:"qualifying"`
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
	if len(stats.Records) != 1 || stats.Records[0].Track != "Italia – Monza" ||
		*stats.Records[0].Race != (lapRecord{81234, "Luca", "Gara"}) ||
		*stats.Records[0].Qualifying != (lapRecord{80100, "Simone", "Qualifica 3"}) {
		t.Errorf("record %+v", stats.Records)
	}
	var monzaRecords struct {
		Race       *lapRecord `json:"race"`
		Qualifying *lapRecord `json:"qualifying"`
	}
	expectStatus(t, call(t, srv, "GET", "/tracks/11/records", nil, &monzaRecords), 200, "record di Monza")
	if monzaRecords.Race == nil || monzaRecords.Race.LapMS != 81234 || monzaRecords.Qualifying == nil || monzaRecords.Qualifying.LapMS != 80100 {
		t.Errorf("record di Monza %+v", monzaRecords)
	}
	monzaRecords.Race, monzaRecords.Qualifying = nil, nil
	expectStatus(t, call(t, srv, "GET", "/tracks/13/records", nil, &monzaRecords), 200, "record di Suzuka")
	if monzaRecords.Race != nil || monzaRecords.Qualifying != nil {
		t.Errorf("record di Suzuka %+v", monzaRecords)
	}
	expectStatus(t, call(t, srv, "GET", "/tracks/x/records", nil, nil), 400, "circuito non valido")
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
		len(profile.BestLaps) != 1 || profile.BestLaps[0].LapMS != 80100 || !profile.BestLaps[0].Record ||
		len(profile.Recent) != 3 || len(profile.Rivals) != 1 || profile.Rivals[0].Ahead != 1 || profile.Rivals[0].Behind != 1 {
		t.Errorf("profilo %+v", profile)
	}
	expectStatus(t, call(t, srv, "GET", "/players/99/stats", nil, nil), 404, "pilota inesistente")

	// Statistiche di un solo torneo: un secondo torneo con una gara vinta da Luca.
	var trophy struct {
		ID int `json:"id"`
	}
	call(t, srv, "POST", "/tournaments", map[string]string{"name": "Trofeo"}, &trophy)
	var spa struct {
		ID int `json:"id"`
	}
	call(t, srv, "POST", fmt.Sprintf("/tournaments/%d/races", trophy.ID), map[string]string{"track": "Belgio – Spa", "type": "race"}, &spa)
	save(spa.ID, res{PlayerID: 2, Position: pos(1)}, res{PlayerID: 1, Position: pos(2)})

	call(t, srv, "GET", fmt.Sprintf("/stats?tournament=%d", cup.ID), nil, &stats)
	if len(stats.Career) != 2 || stats.Career[0].Name != "Luca" || stats.Career[0].Wins != 0 || stats.Career[0].Points != 18+15 ||
		len(stats.Records) != 1 {
		t.Errorf("statistiche della Coppa %+v %+v", stats.Career, stats.Records)
	}
	call(t, srv, "GET", fmt.Sprintf("/stats?tournament=%d", trophy.ID), nil, &stats)
	// Il record di Monza è di una gara della Coppa: nel Trofeo non conta.
	if len(stats.Career) != 2 || stats.Career[0].Name != "Luca" || stats.Career[0].Wins != 1 || stats.Career[0].Races != 1 ||
		len(stats.Records) != 0 || len(stats.HeadToHead.Players) != 2 || stats.HeadToHead.Ahead[0][1] != 1 {
		t.Errorf("statistiche del Trofeo %+v %+v %+v", stats.Career, stats.Records, stats.HeadToHead)
	}
	call(t, srv, "GET", "/stats", nil, &stats)
	if stats.Career[0].Name != "Luca" || stats.Career[0].Wins != 1 || stats.Career[0].Races != 3 {
		t.Errorf("statistiche di tutti i tornei %+v", stats.Career)
	}
	expectStatus(t, call(t, srv, "GET", "/stats?tournament=99", nil, nil), 404, "torneo inesistente")
	expectStatus(t, call(t, srv, "GET", "/stats?tournament=abc", nil, nil), 404, "torneo non valido")
}

type lapRecord struct {
	LapMS       int    `json:"lapMs"`
	Name        string `json:"name"`
	SessionType string `json:"sessionType"`
}
