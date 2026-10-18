package live

import (
	"strings"
	"testing"
	"time"

	"f1-scoreboard/internal/telemetry"
)

func TestTrackerDetailsOnRecording(t *testing.T) {
	var tr Tracker
	for _, p := range loadRecording(t) {
		if h, err := telemetry.ParseHeader(p.Data); err == nil && h.SessionUID != 0 {
			tr.Handle(h, p.Data)
		}
	}
	snap := tr.Snapshot("live")
	const hamilton = 19

	// Direzione gara: sorpassi e contatti solo se c'è di mezzo il giocatore.
	texts := map[string]bool{}
	for i, e := range snap.Events {
		texts[e.Text] = true
		if i > 0 && e.ID != snap.Events[i-1].ID+1 {
			t.Errorf("ID non progressivi: %+v", snap.Events)
		}
		if (e.Kind == KindOvertake || e.Kind == KindCollision) && e.Cars[0] != hamilton && e.Cars[1] != hamilton {
			t.Errorf("evento tra auto della IA: %+v", e)
		}
		if e.Cars == nil {
			t.Errorf("auto nulle: %+v", e)
		}
	}
	for _, want := range []string{
		"Semaforo spento: via!", "Giro più veloce: HAMILTON 1:26.432", "HAMILTON: avvertimento per collisione (con ANTONELLI)",
		"Vince NORRIS", "Bandiera a scacchi",
	} {
		if !texts[want] {
			t.Errorf("manca %q tra %v", want, texts)
		}
	}

	s := snap.Session
	if len(s.MarshalZones) != 17 || len(s.Forecast) != 2 || s.Forecast[0].Offset != 5 {
		t.Errorf("zone %d, previsioni %+v", len(s.MarshalZones), s.Forecast)
	}

	for _, d := range snap.Drivers {
		if d.GridPosition == 0 || d.SpeedTrap < 250 || len(d.Stints) == 0 || d.Stints[0].Tyre == "" {
			t.Errorf("dati del pilota %+v", d)
		}
		if d.Status != "Arrivato" {
			continue
		}
		if sum := d.LastSectors[0] + d.LastSectors[1] + d.LastSectors[2]; sum < d.LastLapMS-2 || sum > d.LastLapMS+2 {
			t.Errorf("%s: settori %v, giro %d", d.Name, d.LastSectors, d.LastLapMS)
		}
		for i, best := range d.BestSectors {
			if best == 0 || best > d.LastSectors[i] {
				t.Errorf("%s: migliori settori %v, ultimi %v", d.Name, d.BestSectors, d.LastSectors)
			}
		}
	}

	car := snap.Players[0]
	if len(car.TyreSets) == 0 || !car.TyreSets[0].Fitted || car.TyreSets[0].Tyre != "S" || car.TyreSets[0].Wear == 0 {
		t.Errorf("set di gomme %+v", car.TyreSets)
	}
	for _, set := range car.TyreSets[1:] {
		if set.Fitted {
			t.Errorf("due set montati: %+v", car.TyreSets)
		}
	}
	if car.Setup == nil || car.Setup.BrakeBias == 0 || car.Setup.TyrePressures[0] == 0 {
		t.Errorf("assetto %+v", car.Setup)
	}

	// Analisi: la griglia e i 5 giri, con l'ultima posizione uguale alla classifica.
	a := tr.Analysis()
	if a == nil || a.Laps != 6 || len(a.Cars) != 20 {
		t.Fatalf("analisi %+v", a)
	}
	humans := 0
	for _, c := range a.Cars {
		if c.Human {
			humans++
		}
		if len(c.Positions) != 6 || c.Positions[0] != int(snapDriver(snap, c.CarIndex).GridPosition) {
			t.Errorf("%s: posizioni %v", c.Name, c.Positions)
		}
		if c.Status == "Arrivato" && (len(c.Laps) != 5 || c.Positions[5] != int(c.Position)) {
			t.Errorf("%s: %d giri, posizioni %v, arrivato %d", c.Name, len(c.Laps), c.Positions, c.Position)
		}
	}
	if humans != 1 {
		t.Errorf("%d giocatori umani", humans)
	}
}

func snapDriver(snap Snapshot, car int) Driver {
	for _, d := range snap.Drivers {
		if d.CarIndex == car {
			return d
		}
	}
	return Driver{}
}

func TestEventFilters(t *testing.T) {
	tr := Tracker{header: telemetry.Header{PlayerCarIndex: 3, SecondaryPlayerCarIndex: 255}}
	event := func(code string, details any) {
		tr.handleEvent(&telemetry.Event{Code: code, Details: details})
	}

	event(telemetry.EventOvertake, telemetry.Overtake{OvertakingVehicleIdx: 1, BeingOvertakenVehicleIdx: 2})
	event(telemetry.EventCollision, telemetry.Collision{Vehicle1Idx: 1, Vehicle2Idx: 2})
	event(telemetry.EventPenalty, telemetry.Penalty{PenaltyType: 5, InfringementType: 7, VehicleIdx: 1, OtherVehicleIdx: 255})
	event(telemetry.EventPenalty, telemetry.Penalty{PenaltyType: 10, InfringementType: 25, VehicleIdx: 3, OtherVehicleIdx: 255})
	event(telemetry.EventRetirement, telemetry.Retirement{VehicleIdx: 1, Reason: 2})
	event(telemetry.EventSafetyCar, telemetry.SafetyCar{Type: 3, EventType: 2})
	event(telemetry.EventButtons, telemetry.Buttons{Status: 1})
	if len(tr.events) != 0 {
		t.Fatalf("eventi da non mostrare: %+v", tr.events)
	}

	event(telemetry.EventOvertake, telemetry.Overtake{OvertakingVehicleIdx: 3, BeingOvertakenVehicleIdx: 2})
	event(telemetry.EventPenalty, telemetry.Penalty{PenaltyType: 4, InfringementType: 17, VehicleIdx: 1, OtherVehicleIdx: 255, Time: 5})
	event(telemetry.EventPenalty, telemetry.Penalty{PenaltyType: 5, InfringementType: 4, VehicleIdx: 3, OtherVehicleIdx: 2})
	event(telemetry.EventSafetyCar, telemetry.SafetyCar{Type: 2, EventType: 0})
	event(telemetry.EventRetirement, telemetry.Retirement{VehicleIdx: 2, Reason: 8})
	want := []string{
		"Auto 4 supera Auto 3",
		"Auto 2: +5 s per eccesso di velocità in corsia box",
		"Auto 4: avvertimento per collisione (con Auto 3)",
		"Virtual safety car: in pista",
		"Ritiro: Auto 3 (guasto meccanico)",
	}
	if len(tr.events) != len(want) {
		t.Fatalf("eventi %+v", tr.events)
	}
	for i, e := range tr.events {
		if e.Text != want[i] || e.ID != i+1 {
			t.Errorf("evento %d: %+v, atteso %q", i, e, want[i])
		}
	}

	// Si tengono gli ultimi maxEvents; eventsSince li restituisce dal primo non ancora visto.
	for range maxEvents {
		event(telemetry.EventFlashback, telemetry.Flashback{})
	}
	if len(tr.events) != maxEvents || tr.events[0].ID != len(want)+1 {
		t.Errorf("%d eventi, il primo è %d", len(tr.events), tr.events[0].ID)
	}
	if got := tr.eventsSince(tr.nextEvent - 2); len(got) != 2 || got[0].ID != tr.nextEvent-1 {
		t.Errorf("eventsSince: %+v", got)
	}
	if got := tr.eventsSince(tr.nextEvent); len(got) != 0 {
		t.Errorf("eventsSince dell'ultimo: %+v", got)
	}
}

func TestInfringementNames(t *testing.T) {
	// La specifica EA ne elenca 55 (0-54): l'ultimo non ha un testo.
	if len(infringements) != 55 || infringements[54] != "" || strings.Contains(strings.Join(infringements[:54], "|"), "||") {
		t.Errorf("%d infrazioni", len(infringements))
	}
}

func TestReplayEventsAndAnalysis(t *testing.T) {
	r := replayOf(t, map[int]string{19: "Simone"})
	info := r.Info()
	if len(info.Events) < 10 {
		t.Fatalf("%d eventi sulla timeline", len(info.Events))
	}
	for i, e := range info.Events {
		if e.AtMS < 0 || e.AtMS > info.DurationMS || (i > 0 && e.AtMS < info.Events[i-1].AtMS) {
			t.Errorf("evento %d fuori ordine o fuori dal replay: %+v", i, e)
		}
	}
	if info.Analysis == nil || info.Analysis.Laps != 6 {
		t.Fatalf("analisi %+v", info.Analysis)
	}
	for _, c := range info.Analysis.Cars {
		if c.CarIndex == 19 && c.PlayerName != "Simone" {
			t.Errorf("pilota del giocatore %+v", c)
		}
	}

	// Il frame finale contiene gli stessi messaggi della timeline (gli ultimi).
	end := frameSnapshot(t, frameAt(t, r, r.Duration+time.Hour))
	if len(end.Events) == 0 || end.Events[len(end.Events)-1].ID != info.Events[len(info.Events)-1].ID {
		t.Errorf("eventi a fine replay %+v", end.Events)
	}
}
