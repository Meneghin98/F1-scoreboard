package api_test

import (
	"fmt"
	"slices"
	"testing"
)

type planState struct {
	rotationState
	Plan []struct {
		Track int `json:"track"`
		Pair  *struct {
			P1 int64 `json:"p1"`
			P2 int64 `json:"p2"`
		} `json:"pair"`
	} `json:"plan"`
}

type trackInfo struct {
	ID       int  `json:"id"`
	Excluded bool `json:"excluded"`
}

func TestTrackPlan(t *testing.T) {
	srv := newTestServer(t)
	for _, name := range []string{"A", "B", "C", "D"} {
		expectStatus(t, call(t, srv, "POST", "/players", map[string]string{"name": name}, nil), 201, "crea pilota")
	}

	// Le piste escluse valgono per tutti i tornei.
	var tracks []trackInfo
	expectStatus(t, call(t, srv, "PUT", "/tracks/excluded", map[string]any{"tracks": []int{5, 31}}, &tracks), 200, "esclude Monaco e Las Vegas")
	expectStatus(t, call(t, srv, "GET", "/tracks", nil, &tracks), 200, "piste")
	var excluded []int
	for _, tr := range tracks {
		if tr.Excluded {
			excluded = append(excluded, tr.ID)
		}
	}
	if fmt.Sprint(excluded) != "[5 31]" {
		t.Fatalf("piste escluse: %v", excluded)
	}
	expectStatus(t, call(t, srv, "PUT", "/tracks/excluded", map[string]any{"tracks": []int{999}}, nil), 400, "pista inesistente")
	expectStatus(t, call(t, srv, "POST", "/rotation/plan", map[string]any{"races": 3}, nil), 409, "programma senza torneo")

	var tour struct {
		ID int64 `json:"id"`
	}
	expectStatus(t, call(t, srv, "POST", "/tournaments", map[string]any{"name": "T", "start": true}, &tour), 201, "crea e inizia")
	available := len(tracks) - 2
	for _, n := range []float64{0, 2.5, float64(available + 1)} {
		expectStatus(t, call(t, srv, "POST", "/rotation/plan", map[string]any{"races": n}, nil), 400, fmt.Sprintf("programma di %v gare", n))
	}

	// Cinque piste diverse, nessuna esclusa, ognuna con la sua coppia.
	var st planState
	expectStatus(t, call(t, srv, "POST", "/rotation/plan", map[string]any{"races": 5}, &st), 200, "genera il programma")
	if len(st.Plan) != 5 || len(st.Next) != 5 {
		t.Fatalf("programma: %+v", st)
	}
	var plan []int
	for i, p := range st.Plan {
		if slices.Contains(plan, p.Track) || p.Track == 5 || p.Track == 31 {
			t.Fatalf("pista %d ripetuta o esclusa nel programma %+v", p.Track, st.Plan)
		}
		plan = append(plan, p.Track)
		if p.Pair == nil || p.Pair.P1 != st.Next[i].P1 || p.Pair.P2 != st.Next[i].P2 {
			t.Fatalf("coppia della pista %d: %+v, attesa %+v", i, p.Pair, st.Next[i])
		}
	}

	// Un Gran Premio corso, su una pista qualunque: il programma avanza di una pista.
	first := st.Next[0]
	call(t, srv, "POST", fmt.Sprintf("/tournaments/%d/races", tour.ID), map[string]string{"track": "Imola", "type": "race"}, nil)
	expectStatus(t, call(t, srv, "PUT", "/races/1/results", map[string]any{"results": []result{
		{PlayerID: int(first.P1), Position: pos(1)}, {PlayerID: int(first.P2), Position: pos(2)},
	}}, nil), 204, "risultati GP")
	expectStatus(t, call(t, srv, "GET", "/rotation", nil, &st), 200, "turnazione dopo il GP")
	if len(st.Plan) != 4 || st.Plan[0].Track != plan[1] {
		t.Fatalf("programma dopo il GP: %+v, prima %v", st.Plan, plan)
	}

	// Con la turnazione spenta il programma resta, senza coppie.
	expectStatus(t, call(t, srv, "PUT", fmt.Sprintf("/tournaments/%d", tour.ID), map[string]any{"rotation": false}, nil), 200, "spegne la turnazione")
	expectStatus(t, call(t, srv, "GET", "/rotation", nil, &st), 200, "turnazione spenta")
	if len(st.Plan) != 4 || st.Plan[0].Pair != nil {
		t.Fatalf("programma con la turnazione spenta: %+v", st.Plan)
	}

	// Un programma nuovo sostituisce il vecchio e riparte dal prossimo Gran Premio.
	expectStatus(t, call(t, srv, "POST", "/rotation/plan", map[string]any{"races": 2}, &st), 200, "nuovo programma")
	if len(st.Plan) != 2 {
		t.Fatalf("nuovo programma: %+v", st.Plan)
	}
	expectStatus(t, call(t, srv, "DELETE", "/rotation/plan", nil, &st), 200, "cancella il programma")
	if len(st.Plan) != 0 {
		t.Fatalf("programma cancellato: %+v", st.Plan)
	}

	// Tutte le piste escluse: non c'è niente da suggerire.
	all := make([]int, len(tracks))
	for i, tr := range tracks {
		all[i] = tr.ID
	}
	expectStatus(t, call(t, srv, "PUT", "/tracks/excluded", map[string]any{"tracks": all}, nil), 200, "esclude tutto")
	expectStatus(t, call(t, srv, "POST", "/rotation/plan", map[string]any{"races": 1}, nil), 400, "programma senza piste")
}
