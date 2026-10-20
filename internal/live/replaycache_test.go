package live

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// raceSession restituisce la gara con la classifica finale della registrazione di prova.
func raceSession(t *testing.T, s *Service) SessionRecord {
	t.Helper()
	records, err := s.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range records {
		if r.IsRace && r.HasClassification {
			return r
		}
	}
	t.Fatalf("gara non trovata tra %d sessioni", len(records))
	return SessionRecord{}
}

// cacheHeader legge l'intestazione del replay salvato in path.
func cacheHeader(t *testing.T, path string) replayHeader {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var h replayHeader
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&h); err != nil {
		t.Fatal(err)
	}
	return h
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Il replay preparato si salva accanto alla registrazione e dopo si rilegge da
// lì, identico; con un altro eseguibile o altri piloti si rifà.
func TestReplayCache(t *testing.T) {
	s, dir := recordedService(t)
	race := raceSession(t, s)
	cache := filepath.Join(dir, race.file+replayExt)

	built, err := s.Replay(race.ID)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(cache)
	if err != nil {
		t.Fatalf("replay non salvato: %v", err)
	}
	t.Logf("replay salvato: %d KB", info.Size()/1024)

	s.forgetReplay(race.ID)
	start := time.Now()
	loaded, err := s.Replay(race.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("replay riletto dal file in %s", time.Since(start).Round(time.Millisecond))
	if loaded == built || loaded.Duration != built.Duration || len(loaded.frames) != len(built.frames) {
		t.Fatalf("replay riletto: %s, %d fotogrammi", loaded.Duration, len(loaded.frames))
	}
	if mustJSON(t, loaded.Info()) != mustJSON(t, built.Info()) || mustJSON(t, loaded.TrackMap()) != mustJSON(t, built.TrackMap()) {
		t.Error("informazioni o pianta diverse da quelle del replay preparato")
	}
	for _, at := range []time.Duration{0, built.Duration / 3, built.Duration / 2, built.Duration} {
		if mustJSON(t, frameAt(t, loaded, at)) != mustJSON(t, frameAt(t, built, at)) {
			t.Errorf("fotogramma a %s diverso", at)
		}
	}

	// Un replay scritto da un altro eseguibile non vale: si rifà e si sovrascrive.
	old := cacheHeader(t, cache)
	stale := old
	stale.Executable = "versione precedente"
	if err := saveReplay(cache, stale, built); err != nil {
		t.Fatal(err)
	}
	s.forgetReplay(race.ID)
	if _, err := s.Replay(race.ID); err != nil {
		t.Fatal(err)
	}
	if h := cacheHeader(t, cache); h.Executable != executableID() {
		t.Errorf("replay non rifatto: eseguibile %q", h.Executable)
	}

	// Cambiando i piloti il replay salvato si elimina; cambiando il nome di un
	// pilota non vale più, e il replay rifatto mostra quello nuovo.
	simone := insertID(t, s, "INSERT INTO players (name) VALUES ('Simone')")
	car := race.Players[0].CarIndex
	if err := s.SetSessionPlayers(race.ID, map[int]*int64{car: &simone}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("replay salvato non eliminato cambiando i piloti: %v", err)
	}
	if _, err := s.Replay(race.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("UPDATE players SET name = 'Simone M.' WHERE id = ?", simone); err != nil {
		t.Fatal(err)
	}
	s.forgetReplay(race.ID)
	r, err := s.Replay(race.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range frameSnapshot(t, frameAt(t, r, r.Duration)).Drivers {
		found = found || (d.CarIndex == car && d.PlayerName == "Simone M.")
	}
	if !found {
		t.Error("il replay non mostra il nuovo nome del pilota")
	}

	// Eliminando la sessione se ne va anche il replay salvato.
	if err := s.DeleteSession(race.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Errorf("replay salvato non eliminato con la sessione: %v", err)
	}
}

// Finita una sessione del weekend, il replay si prepara e si salva da solo.
func TestPrepareReplayAtSessionEnd(t *testing.T) {
	packets := loadRecording(t)
	s, dir := newTestService(t, true)
	s.opts.PrepareReplays = true
	base := time.Date(2026, 10, 16, 13, 31, 38, 0, time.UTC)
	for _, p := range packets {
		s.Handle(base.Add(p.At), p.Data)
	}
	s.tick(s.lastPacket.Add(sessionTimeout + time.Second))
	s.background.Wait()

	records, err := s.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range records {
		_, err := os.Stat(filepath.Join(dir, r.file+replayExt))
		if want := r.HasClassification; (err == nil) != want {
			t.Errorf("sessione %d (classifica finale %v): replay salvato %v", r.ID, r.HasClassification, err == nil)
		}
	}
	if s.replays[raceSession(t, s).ID] != nil {
		t.Error("il replay preparato a fine sessione non deve restare in memoria")
	}
}

// Allo spegnimento del server la preparazione di un replay si interrompe.
func TestReplayStopsOnShutdown(t *testing.T) {
	s, _ := recordedService(t)
	race := raceSession(t, s)
	close(s.quit)
	if _, err := s.Replay(race.ID); !errors.Is(err, errStopping) {
		t.Fatalf("replay durante lo spegnimento: %v", err)
	}
}
