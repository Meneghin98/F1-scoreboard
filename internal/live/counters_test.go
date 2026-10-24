package live

import (
	"slices"
	"testing"
	"time"

	"f1-scoreboard/internal/recording"
	"f1-scoreboard/internal/telemetry"
)

// La registrazione di prova ha la gara di Monza ricominciata quattro volte
// prima di quella finita: l'ultima sessione conta 4 riavvii. Due flashback
// aggiunti nella gara finita si contano dal vivo e rileggendo il file.
func TestSessionCounters(t *testing.T) {
	packets := loadRecording(t)
	var last uint64
	for _, p := range packets {
		if h, err := telemetry.ParseHeader(p.Data); err == nil && h.SessionUID != 0 {
			last = h.SessionUID
		}
	}
	at := packets[len(packets)/2].At // a metà registrazione si corre l'ultima gara
	flashback := recording.Packet{At: at, Data: telemetryPacket(t, telemetry.PacketEvent, last, telemetry.EventFlashback)}
	i := len(packets) / 2
	packets = slices.Insert(packets, i, flashback, flashback)

	s, _ := newTestService(t, true)
	base := time.Date(2026, 10, 16, 13, 31, 38, 0, time.UTC)
	for _, p := range packets {
		s.Handle(base.Add(p.At), p.Data)
	}
	s.tick(s.lastPacket.Add(sessionTimeout + time.Second))

	check := func(what string) {
		t.Helper()
		var restarts, flashbacks []int
		rows, err := s.db.Query("SELECT COALESCE(restarts, -1), COALESCE(flashbacks, -1) FROM telemetry_sessions ORDER BY id")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var r, f int
			if err := rows.Scan(&r, &f); err != nil {
				t.Fatal(err)
			}
			restarts, flashbacks = append(restarts, r), append(flashbacks, f)
		}
		if !slices.Equal(restarts, []int{0, 1, 2, 3, 4}) || !slices.Equal(flashbacks, []int{0, 0, 0, 0, 2}) {
			t.Fatalf("%s: riavvii %v, flashback %v", what, restarts, flashbacks)
		}
	}
	check("dal vivo")

	// Sessioni registrate prima dei contatori: si ricalcolano all'avvio.
	s.exec("UPDATE telemetry_sessions SET restarts = NULL, flashbacks = NULL")
	s.backfillCounters()
	check("dopo la rilettura")

	// La gara del torneo li mostra con la sua registrazione.
	records, _ := s.Sessions()
	recs, err := s.TournamentRecordings(records[0].Race.TournamentID)
	if err != nil {
		t.Fatal(err)
	}
	rec := recs[records[0].Race.ID][0]
	if rec.Restarts == nil || *rec.Restarts != 4 || rec.Flashbacks == nil || *rec.Flashbacks != 2 {
		t.Fatalf("registrazione della gara %+v", rec)
	}
}

// Una sessione nuova dopo una finita, o su un altro circuito, non è un riavvio.
func TestRestartNeedsSameUnfinishedSession(t *testing.T) {
	s, _ := newTestService(t, false)
	add := func(uid string, track, kind int, classification bool, start, end string) int64 {
		return insertID(t, s, `INSERT INTO telemetry_sessions (session_uid, packet_format, track_id, session_type,
			has_classification, started_at, ended_at, file) VALUES (?, 2025, ?, ?, ?, ?, ?, '')`,
			uid, track, kind, classification, start, end)
	}
	restarts := func(id int64) int {
		t.Helper()
		n, err := restartsOf(s.db, id)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	add("a", 11, 15, true, "2026-10-16 14:00:00", "2026-10-16 14:10:00")
	if n := restarts(add("b", 11, 15, false, "2026-10-16 14:10:05", "2026-10-16 14:11:00")); n != 0 {
		t.Errorf("dopo una gara finita: %d riavvii", n)
	}
	if n := restarts(add("c", 3, 15, false, "2026-10-16 14:11:05", "2026-10-16 14:12:00")); n != 0 {
		t.Errorf("su un altro circuito: %d riavvii", n)
	}
	if n := restarts(add("d", 3, 15, false, "2026-10-16 14:30:00", "2026-10-16 14:31:00")); n != 0 {
		t.Errorf("molto dopo: %d riavvii", n)
	}
}
