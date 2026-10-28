package api_test

import (
	"fmt"
	"testing"
)

type rotationState struct {
	Tournament *struct {
		ID int64 `json:"id"`
	} `json:"tournament"`
	Present []int64 `json:"present"`
	Enabled bool    `json:"enabled"`
	CanDraw bool    `json:"canDraw"`
	Next    []struct {
		P1 int64 `json:"p1"`
		P2 int64 `json:"p2"`
	} `json:"next"`
}

func TestRotation(t *testing.T) {
	srv, db := newTestServerDB(t)
	for _, name := range []string{"A", "B", "C", "D"} {
		expectStatus(t, call(t, srv, "POST", "/players", map[string]string{"name": name}, nil), 201, "crea pilota")
	}

	// Di default sono tutti presenti.
	var st rotationState
	expectStatus(t, call(t, srv, "GET", "/rotation", nil, &st), 200, "turnazione iniziale")
	if fmt.Sprint(st.Present) != "[1 2 3 4]" {
		t.Fatalf("presenti di default: %v", st.Present)
	}

	// Senza torneo in corso i presenti si scelgono lo stesso, ma non ci sono coppie.
	expectStatus(t, call(t, srv, "PUT", "/rotation/present", map[string]any{"players": []int64{2, 3}}, &st), 200, "presenti")
	if fmt.Sprint(st.Present) != "[2 3]" {
		t.Fatalf("presenti scelti: %v", st.Present)
	}
	expectStatus(t, call(t, srv, "PUT", "/rotation/present", map[string]any{"players": []int64{4, 2, 1, 3, 2}}, &st), 200, "presenti")
	if st.Tournament != nil || len(st.Next) != 0 || fmt.Sprint(st.Present) != "[1 2 3 4]" {
		t.Fatalf("turnazione senza torneo: %+v", st)
	}
	expectStatus(t, call(t, srv, "PUT", "/rotation/present", map[string]any{"players": []int64{1, 99}}, nil), 400, "pilota inesistente")
	expectStatus(t, call(t, srv, "POST", "/rotation/draw", nil, nil), 409, "sorteggio senza torneo")

	// All'avvio del torneo l'ordine si sorteggia da solo.
	var tour struct {
		ID int64 `json:"id"`
	}
	expectStatus(t, call(t, srv, "POST", "/tournaments", map[string]any{"name": "T", "start": true}, &tour), 201, "crea e inizia")
	var order string
	if err := db.QueryRow("SELECT rotation_order FROM tournaments WHERE id = ?", tour.ID).Scan(&order); err != nil {
		t.Fatal(err)
	}
	if len(order) != len("1,2,3,4") {
		t.Fatalf("ordine sorteggiato all'avvio: %q", order)
	}

	expectStatus(t, call(t, srv, "GET", "/rotation", nil, &st), 200, "turnazione")
	if st.Tournament == nil || st.Tournament.ID != tour.ID || !st.Enabled || !st.CanDraw || len(st.Next) != 3 {
		t.Fatalf("turnazione del torneo: %+v", st)
	}
	expectStatus(t, call(t, srv, "POST", "/rotation/draw", nil, &st), 200, "nuovo sorteggio")

	// Si corre la prima coppia proposta: dopo non si sorteggia più e la coppia successiva cambia.
	first := st.Next[0]
	call(t, srv, "POST", fmt.Sprintf("/tournaments/%d/races", tour.ID), map[string]string{"track": "Monza", "type": "race"}, nil)
	expectStatus(t, call(t, srv, "PUT", "/races/1/results", map[string]any{"results": []result{
		{PlayerID: int(first.P1), Position: pos(1)}, {PlayerID: int(first.P2), Position: pos(2)},
	}}, nil), 204, "risultati GP")
	expectStatus(t, call(t, srv, "GET", "/rotation", nil, &st), 200, "turnazione dopo il GP")
	if st.CanDraw {
		t.Error("dopo il primo GP il sorteggio non si può più rifare")
	}
	for _, id := range []int64{st.Next[0].P1, st.Next[0].P2} {
		if id == first.P1 || id == first.P2 {
			t.Errorf("dopo %v la coppia successiva dovrebbe essere degli altri due: %v", first, st.Next[0])
		}
	}
	expectStatus(t, call(t, srv, "POST", "/rotation/draw", nil, nil), 409, "sorteggio dopo il primo GP")

	// Le prove e le qualifiche non fanno avanzare la turnazione.
	before := st.Next[0]
	call(t, srv, "POST", fmt.Sprintf("/tournaments/%d/races", tour.ID), map[string]string{"track": "Imola", "type": "qualifying"}, nil)
	expectStatus(t, call(t, srv, "PUT", "/races/2/results", map[string]any{"results": []result{
		{PlayerID: int(before.P1), Position: pos(1)}, {PlayerID: int(before.P2), Position: pos(2)},
	}}, nil), 204, "risultati qualifica")
	expectStatus(t, call(t, srv, "GET", "/rotation", nil, &st), 200, "turnazione dopo la qualifica")
	if st.Next[0] != before {
		t.Errorf("dopo la qualifica la coppia è cambiata: %v, prima %v", st.Next[0], before)
	}

	// Un pilota nuovo è presente da subito; eliminato, sparisce dai presenti.
	expectStatus(t, call(t, srv, "POST", "/players", map[string]string{"name": "E"}, nil), 201, "crea pilota")
	expectStatus(t, call(t, srv, "GET", "/rotation", nil, &st), 200, "turnazione con E")
	if fmt.Sprint(st.Present) != "[1 2 3 4 5]" {
		t.Errorf("presenti con il pilota nuovo: %v", st.Present)
	}
	expectStatus(t, call(t, srv, "DELETE", "/players/5", nil, nil), 204, "elimina pilota")
	expectStatus(t, call(t, srv, "GET", "/rotation", nil, &st), 200, "turnazione dopo l'eliminazione")
	if fmt.Sprint(st.Present) != "[1 2 3 4]" {
		t.Errorf("presenti dopo l'eliminazione: %v", st.Present)
	}

	// La turnazione si spegne nelle impostazioni del torneo: niente coppie né
	// sorteggio, ma i presenti restano; riaccesa, riparte dallo storico.
	var updated struct {
		Rotation bool `json:"rotation"`
	}
	expectStatus(t, call(t, srv, "PUT", fmt.Sprintf("/tournaments/%d", tour.ID), map[string]any{"rotation": false}, &updated), 200, "spegne la turnazione")
	if updated.Rotation {
		t.Fatal("il torneo ha ancora la turnazione attiva")
	}
	expectStatus(t, call(t, srv, "GET", "/rotation", nil, &st), 200, "turnazione spenta")
	if st.Tournament == nil || st.Enabled || st.CanDraw || len(st.Next) != 0 || fmt.Sprint(st.Present) != "[1 2 3 4]" {
		t.Fatalf("turnazione spenta: %+v", st)
	}
	expectStatus(t, call(t, srv, "POST", "/rotation/draw", nil, nil), 409, "sorteggio con la turnazione spenta")
	expectStatus(t, call(t, srv, "PUT", fmt.Sprintf("/tournaments/%d", tour.ID), map[string]any{"name": "T2"}, &updated), 200, "rinomina")
	if updated.Rotation {
		t.Fatal("cambiando il nome la turnazione si è riaccesa")
	}
	expectStatus(t, call(t, srv, "PUT", fmt.Sprintf("/tournaments/%d", tour.ID), map[string]any{"rotation": true}, nil), 200, "riaccende la turnazione")
	expectStatus(t, call(t, srv, "GET", "/rotation", nil, &st), 200, "turnazione riaccesa")
	if !st.Enabled || len(st.Next) != 3 {
		t.Fatalf("turnazione riaccesa: %+v", st)
	}
}

// Un torneo si può creare con la turnazione spenta; di default è attiva.
func TestRotationDisabledAtCreation(t *testing.T) {
	srv := newTestServer(t)
	var tour struct {
		Rotation bool `json:"rotation"`
	}
	expectStatus(t, call(t, srv, "POST", "/tournaments", map[string]any{"name": "A"}, &tour), 201, "crea")
	if !tour.Rotation {
		t.Error("di default la turnazione dovrebbe essere attiva")
	}
	expectStatus(t, call(t, srv, "POST", "/tournaments", map[string]any{"name": "B", "start": true, "rotation": false}, &tour), 201, "crea senza turnazione")
	if tour.Rotation {
		t.Error("il torneo è stato creato con la turnazione attiva")
	}
	var st rotationState
	expectStatus(t, call(t, srv, "GET", "/rotation", nil, &st), 200, "turnazione")
	if st.Tournament == nil || st.Enabled || len(st.Next) != 0 {
		t.Fatalf("turnazione del torneo senza turnazione: %+v", st)
	}
}
