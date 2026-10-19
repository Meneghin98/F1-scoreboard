package live

import (
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"f1-scoreboard/internal/recording"
	"f1-scoreboard/internal/telemetry"
)

func replayOf(t *testing.T, names map[int]string) *Replay {
	t.Helper()
	players := map[int]scoreboardPlayer{}
	for car, name := range names {
		players[car] = scoreboardPlayer{Name: name, Color: "#3987e5"}
	}
	packets := loadRecording(t)
	// La registrazione di prova contiene più sessioni: si usa la gara, quella con più pacchetti.
	counts := map[uint64]int{}
	var uid uint64
	for _, p := range packets {
		if h, err := telemetry.ParseHeader(p.Data); err == nil && h.SessionUID != 0 {
			counts[h.SessionUID]++
			if counts[h.SessionUID] > counts[uid] {
				uid = h.SessionUID
			}
		}
	}
	i := 0
	next := func() (recording.Packet, error) {
		if i == len(packets) {
			return recording.Packet{}, io.EOF
		}
		i++
		return packets[i-1], nil
	}
	start := time.Now()
	r, err := buildReplay(next, uid, players, func(*telemetry.Session) *outline { return nil })
	if err != nil {
		t.Fatal(err)
	}
	size := 0
	for _, f := range r.frames {
		size += len(f.data)
	}
	t.Logf("replay di %s: %d fotogrammi (%d KB), %d campioni dei pedali, preparato in %s",
		r.Duration.Round(time.Second), len(r.frames), size/1024, len(r.pedals), time.Since(start).Round(time.Millisecond))
	return r
}

func frameSnapshot(t *testing.T, f ReplayFrame) Snapshot {
	t.Helper()
	var snap Snapshot
	if err := json.Unmarshal(f.Snapshot, &snap); err != nil {
		t.Fatal(err)
	}
	return snap
}

func TestReplayOnRecording(t *testing.T) {
	r := replayOf(t, map[int]string{})
	if r.Duration < time.Minute {
		t.Fatalf("durata %s", r.Duration)
	}

	// Alla fine si vede la classifica finale, come nella pagina Live.
	end := frameSnapshot(t, frameAt(t, r, r.Duration+time.Hour))
	if end.Session == nil || end.Session.Track != "Monza" || !end.Finished || len(end.Drivers) != 20 {
		t.Fatalf("fine del replay: sessione %+v, finita %v, %d piloti", end.Session, end.Finished, len(end.Drivers))
	}
	if end.Session.MapPoints != 0 {
		t.Errorf("mapPoints %d: la pianta del replay non deve essere riscaricata", end.Session.MapPoints)
	}

	// Il replay parte quando la sessione è caricata: circuito e piloti ci sono subito.
	if start := frameSnapshot(t, frameAt(t, r, 0)); start.Session == nil || start.Session.Track != "Monza" || len(start.Drivers) == 0 {
		t.Errorf("inizio del replay: sessione %+v, %d piloti", start.Session, len(start.Drivers))
	}

	// A metà gara non è ancora finita, e tornando indietro lo stato è quello di allora.
	mid := frameAt(t, r, r.Duration/2)
	snap := frameSnapshot(t, mid)
	if snap.Finished || snap.Session == nil || len(snap.Drivers) == 0 {
		t.Fatalf("metà del replay: finita %v, sessione %+v", snap.Finished, snap.Session)
	}
	if mid.T != (r.Duration / 2).Milliseconds() {
		t.Errorf("t = %d", mid.T)
	}
	if string(frameAt(t, r, r.Duration/2).Snapshot) != string(mid.Snapshot) {
		t.Error("lo stesso istante deve dare lo stesso stato")
	}

	// Lo storico dei pedali copre gli ultimi pedalWindow e finisce all'istante richiesto.
	if len(mid.Pedals) == 0 {
		t.Fatal("nessun dato dei pedali a metà gara")
	}
	first, last := mid.Pedals[0], mid.Pedals[len(mid.Pedals)-1]
	if last.T > mid.T || mid.T-first.T > pedalWindow.Milliseconds() {
		t.Errorf("pedali da %d a %d per t = %d", first.T, last.T, mid.T)
	}
	if first.Controller != 1 {
		t.Errorf("pedali del controller %d, atteso 1", first.Controller)
	}

	// Una tacca per giro, in ordine.
	if len(r.laps) < 5 {
		t.Fatalf("%d giri sulla timeline: %+v", len(r.laps), r.laps)
	}
	for i, l := range r.laps {
		if l.Lap != i+1 || (i > 0 && l.AtMS <= r.laps[i-1].AtMS) {
			t.Errorf("giri %+v", r.laps)
			break
		}
	}

	if m := r.TrackMap(); m == nil || len(m.Points) < 100 {
		t.Errorf("pianta del circuito mancante")
	}
	if info := r.Info(); info.MapPoints == 0 || info.DurationMS != r.Duration.Milliseconds() {
		t.Errorf("info %+v", info)
	}

	// Prima dell'inizio non c'è niente da mostrare ma la risposta è valida.
	if f := frameAt(t, r, -time.Second); f.T != 0 || f.Pedals == nil {
		t.Errorf("inizio: %+v", f)
	}
}

func TestReplayShowsPlayerNames(t *testing.T) {
	var player int
	for _, d := range frameSnapshot(t, frameAt(t, replayOf(t, nil), time.Hour)).Drivers {
		if d.Controller > 0 {
			player = d.CarIndex
		}
	}
	snap := frameSnapshot(t, frameAt(t, replayOf(t, map[int]string{player: "Mario"}), time.Hour))
	for _, d := range snap.Drivers {
		if want := map[bool]string{true: "Mario"}[d.CarIndex == player]; d.PlayerName != want {
			t.Errorf("auto %d: pilota %q, atteso %q", d.CarIndex, d.PlayerName, want)
		}
	}
}

func frameAt(t *testing.T, r *Replay, at time.Duration) ReplayFrame {
	t.Helper()
	f, err := r.Frame(at)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestServiceReplay(t *testing.T) {
	s, _ := recordedService(t)
	records, err := s.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	var race SessionRecord
	for _, r := range records {
		if r.IsRace && r.HasClassification {
			race = r
		}
	}
	if race.ID == 0 || len(race.Players) == 0 {
		t.Fatalf("gara non trovata tra %d sessioni", len(records))
	}

	r, err := s.Replay(race.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := s.Replay(race.ID); again != r {
		t.Error("il replay va ripreso dalla memoria, non ripreparato")
	}
	if !frameSnapshot(t, frameAt(t, r, r.Duration)).Finished {
		t.Error("alla fine del replay la gara deve essere finita")
	}

	// Cambiando i piloti il replay si riprepara con i nomi nuovi.
	simone := insertID(t, s, "INSERT INTO players (name) VALUES ('Simone')")
	car := race.Players[0].CarIndex
	if err := s.SetSessionPlayers(race.ID, map[int]*int64{car: &simone}); err != nil {
		t.Fatal(err)
	}
	r, err = s.Replay(race.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range frameSnapshot(t, frameAt(t, r, r.Duration)).Drivers {
		found = found || (d.CarIndex == car && d.PlayerName == "Simone")
	}
	if !found {
		t.Error("il replay non mostra il pilota scelto")
	}

	if err := s.DeleteSession(race.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Replay(race.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("replay di una sessione eliminata: %v", err)
	}
	if _, err := s.Replay(999); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("replay di una sessione inesistente: %v", err)
	}
}

func TestReplayOfSessionInProgress(t *testing.T) {
	packets := loadRecording(t)
	s, _ := newTestService(t, true)
	for _, p := range packets[:5000] {
		s.Handle(time.Now(), p.Data)
	}
	records, _ := s.Sessions()
	if _, err := s.Replay(records[0].ID); !errors.Is(err, ErrSessionRecording) {
		t.Fatalf("replay della sessione in corso: %v", err)
	}
}
