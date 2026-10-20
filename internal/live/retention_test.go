package live

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Dopo i giorni scelti si eliminano i file delle registrazioni, non le sessioni;
// quelli delle gare dei tornei restano, se richiesto.
func TestDeleteOldRecordings(t *testing.T) {
	s, dir := newTestService(t, false)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	cup := insertID(t, s, "INSERT INTO tournaments (name) VALUES ('Coppa')")
	cupRace := insertID(t, s, "INSERT INTO races (tournament_id, track, type) VALUES (?, 'Monza', 'race')", cup)
	freeRace := insertID(t, s, "INSERT INTO races (tournament_id, track, type) VALUES ((SELECT id FROM tournaments WHERE free = 1), 'Monza', 'race')")

	// session crea una sessione finita da days giorni, con il suo file.
	session := func(name string, days int, raceID any, analyzed bool) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), make([]byte, 1000), 0o644); err != nil {
			t.Fatal(err)
		}
		ended := dbTime(now.AddDate(0, 0, -days))
		insertID(t, s, `INSERT INTO telemetry_sessions (session_uid, packet_format, started_at, ended_at, file, analyzed, race_id)
			VALUES ('1', 2025, ?, ?, ?, ?, ?)`, ended, ended, name, analyzed, raceID)
	}
	session("vecchia.f1rec.gz", 40, nil, true)
	session("torneo.f1rec.gz", 40, cupRace, true)
	session("nessun-torneo.f1rec.gz", 40, freeRace, true)
	session("recente.f1rec.gz", 5, nil, true)
	session("da-analizzare.f1rec.gz", 40, nil, false)
	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(dir, name))
		return err == nil
	}

	// Di default si tiene tutto.
	if deleted, _ := s.deleteOldRecordings(now); deleted != 0 {
		t.Fatalf("eliminati %d file senza impostazioni", deleted)
	}
	// Salvate le impostazioni, i file troppo vecchi si eliminano subito.
	deleted, freed, err := s.SetRetention(Retention{Days: 30, KeepTournaments: true})
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 2 || freed != 2000 || exists("vecchia.f1rec.gz") || exists("nessun-torneo.f1rec.gz") {
		t.Fatalf("eliminati %d file (%d byte)", deleted, freed)
	}
	if !exists("torneo.f1rec.gz") || !exists("recente.f1rec.gz") || !exists("da-analizzare.f1rec.gz") {
		t.Fatal("eliminato un file da tenere")
	}
	if n := countRows(t, s, "SELECT COUNT(*) FROM telemetry_sessions"); n != 5 {
		t.Fatalf("%d sessioni, attese 5: le righe restano", n)
	}

	// Senza tenere le gare dei tornei va anche quella del torneo.
	deleted, _, err = s.SetRetention(Retention{Days: 30})
	if err != nil || deleted != 1 || exists("torneo.f1rec.gz") {
		t.Fatalf("eliminati %d file, %v", deleted, err)
	}
	if r, err := s.Retention(); err != nil || r.Days != 30 || r.KeepTournaments {
		t.Fatalf("impostazioni %+v, %v", r, err)
	}
	// Il controllo di ogni ora non trova altro da eliminare.
	if deleted, _ := s.deleteOldRecordings(now.Add(time.Hour)); deleted != 0 {
		t.Fatalf("eliminati altri %d file", deleted)
	}

	files, size, err := s.RecordingsUsage()
	if err != nil || files != 2 || size != 2000 {
		t.Fatalf("cartella delle registrazioni: %d file, %d byte, %v", files, size, err)
	}
}
