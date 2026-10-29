package live

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"f1-scoreboard/internal/telemetry"
)

// Dopo i giorni scelti si eliminano i file delle registrazioni, non le sessioni;
// quelli delle sessioni che detengono un record restano, se richiesto.
func TestDeleteOldRecordings(t *testing.T) {
	s, dir := newTestService(t, false)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	cup := insertID(t, s, "INSERT INTO tournaments (name) VALUES ('Coppa')")
	cupRace := insertID(t, s, "INSERT INTO races (tournament_id, track, type) VALUES (?, 'Monza', 'race')", cup)
	luca := insertID(t, s, "INSERT INTO players (name) VALUES ('Luca')")

	// session crea una sessione finita da days giorni, con il suo file e, se
	// lapMS > 0, il miglior giro di Luca a Monza.
	session := func(name string, days int, raceID any, analyzed bool, st telemetry.SessionType, lapMS int) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), make([]byte, 1000), 0o644); err != nil {
			t.Fatal(err)
		}
		ended := dbTime(now.AddDate(0, 0, -days))
		id := insertID(t, s, `INSERT INTO telemetry_sessions (session_uid, packet_format, started_at, ended_at, file, analyzed, race_id, track_id, session_type)
			VALUES ('1', 2025, ?, ?, ?, ?, ?, 11, ?)`, ended, ended, name, analyzed, raceID, int(st))
		if lapMS > 0 {
			if _, err := s.db.Exec(`INSERT INTO telemetry_players (session_id, car_index, name, team, race_number, player_id, best_lap_ms)
				VALUES (?, 0, 'LUCA', 'Ferrari', 16, ?, ?)`, id, luca, lapMS); err != nil {
				t.Fatal(err)
			}
		}
	}
	session("vecchia.f1rec.gz", 40, nil, true, telemetry.SessionPractice1, 0)
	session("torneo.f1rec.gz", 40, cupRace, true, telemetry.SessionRace, 81000)
	session("record-gara.f1rec.gz", 40, cupRace, true, telemetry.SessionRace, 80000)
	session("record-qualifica.f1rec.gz", 40, nil, true, telemetry.SessionQualifying3, 79000)
	session("prova-veloce.f1rec.gz", 40, nil, true, telemetry.SessionPractice1, 70000) // le prove non fanno record
	session("recente.f1rec.gz", 5, nil, true, telemetry.SessionPractice1, 0)
	session("da-analizzare.f1rec.gz", 40, nil, false, telemetry.SessionPractice1, 0)
	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(dir, name))
		return err == nil
	}

	// Di default si tiene tutto.
	if deleted, _ := s.deleteOldRecordings(now); deleted != 0 {
		t.Fatalf("eliminati %d file senza impostazioni", deleted)
	}
	if r, err := s.Retention(); err != nil || r.Days != 0 || !r.KeepRecords {
		t.Fatalf("impostazioni di default %+v, %v", r, err)
	}
	// Salvate le impostazioni, i file troppo vecchi si eliminano subito, anche
	// quelli delle gare dei tornei, ma non quelli dei record.
	deleted, freed, err := s.SetRetention(Retention{Days: 30, KeepRecords: true})
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 3 || freed != 3000 || exists("vecchia.f1rec.gz") || exists("torneo.f1rec.gz") || exists("prova-veloce.f1rec.gz") {
		t.Fatalf("eliminati %d file (%d byte)", deleted, freed)
	}
	for _, name := range []string{"record-gara.f1rec.gz", "record-qualifica.f1rec.gz", "recente.f1rec.gz", "da-analizzare.f1rec.gz"} {
		if !exists(name) {
			t.Fatalf("eliminato %s, da tenere", name)
		}
	}
	if n := countRows(t, s, "SELECT COUNT(*) FROM telemetry_sessions"); n != 7 {
		t.Fatalf("%d sessioni, attese 7: le righe restano", n)
	}

	// Senza tenere i record vanno anche quelli.
	deleted, _, err = s.SetRetention(Retention{Days: 30})
	if err != nil || deleted != 2 || exists("record-gara.f1rec.gz") || exists("record-qualifica.f1rec.gz") {
		t.Fatalf("eliminati %d file, %v", deleted, err)
	}
	if r, err := s.Retention(); err != nil || r.Days != 30 || r.KeepRecords {
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
