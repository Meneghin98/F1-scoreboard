package api_test

import (
	"testing"

	"f1-scoreboard/internal/api"
)

// Gli assetti consigliati arrivano dalle migrazioni: l'API li restituisce per
// id del circuito, solo per i circuiti che ne hanno uno. Il test non dipende
// dagli assetti già inseriti: usa Monza, che non ne ha.
func TestTrackSetups(t *testing.T) {
	srv, db := newTestServerDB(t)

	var setups map[int]api.TrackSetup
	expectStatus(t, call(t, srv, "GET", "/tracks/setups", nil, &setups), 200, "assetti delle migrazioni")
	if _, ok := setups[11]; ok {
		t.Fatalf("Monza ha già un assetto: il test va spostato su un altro circuito")
	}
	before := len(setups)

	if _, err := db.Exec(`INSERT INTO track_setups (track_id, front_wing, rear_wing, on_throttle, off_throttle, engine_braking,
		front_camber, rear_camber, front_toe, rear_toe,
		front_suspension, rear_suspension, front_anti_roll_bar, rear_anti_roll_bar, front_height, rear_height,
		brake_pressure, brake_bias,
		front_left_pressure, front_right_pressure, rear_left_pressure, rear_right_pressure, notes)
		VALUES (11, 3, 5, 100, 60, 90, -3.5, -2, 0.05, 0.2, 40, 10, 18, 6, 20, 45, 100, 56, 29.5, 29.4, 26.5, 26.4, 'Monza')`); err != nil {
		t.Fatal(err)
	}
	expectStatus(t, call(t, srv, "GET", "/tracks/setups", nil, &setups), 200, "assetti")
	s, ok := setups[11]
	if len(setups) != before+1 || !ok {
		t.Fatalf("assetti %v, atteso in più Monza", setups)
	}
	want := api.TrackSetup{
		FrontWing: 3, RearWing: 5, OnThrottle: 100, OffThrottle: 60,
		FrontCamber: -3.5, RearCamber: -2, FrontToe: 0.05, RearToe: 0.2,
		FrontSuspension: 40, RearSuspension: 10, FrontAntiRollBar: 18, RearAntiRollBar: 6, FrontHeight: 20, RearHeight: 45,
		BrakePressure: 100, BrakeBias: 56, TyrePressures: [4]float64{29.5, 29.4, 26.5, 26.4}, Notes: "Monza",
	}
	if s.EngineBraking == nil || *s.EngineBraking != 90 {
		t.Errorf("freno motore di Monza %v, atteso 90", s.EngineBraking)
	}
	s.EngineBraking = nil
	if s != want {
		t.Errorf("assetto di Monza %+v, atteso %+v", s, want)
	}
}
