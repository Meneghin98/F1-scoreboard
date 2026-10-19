package live

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"f1-scoreboard/internal/telemetry"
)

// Una sessione finita che il server non ha fatto in tempo a salvare (spento
// di colpo) finisce nel torneo in corso al riavvio, una volta sola.
func TestRecoverPending(t *testing.T) {
	s, _ := recordedService(t)
	records, _ := s.Sessions()
	last := records[0]
	var pending bool
	s.db.QueryRow("SELECT pending_save FROM telemetry_sessions WHERE id = ?", last.ID).Scan(&pending)
	if last.Race == nil || pending {
		t.Fatalf("dopo il salvataggio: gara %+v, in sospeso %v", last.Race, pending)
	}

	// Come se il server si fosse spento prima di salvarla nel torneo.
	s.exec("DELETE FROM races")
	s.exec("UPDATE telemetry_sessions SET pending_save = 1 WHERE id = ?", last.ID)
	cup := insertID(t, s, "INSERT INTO tournaments (name, started_at) VALUES ('Campionato', datetime('now'))")
	s.recoverPending()
	s.recoverPending() // la seconda volta non c'è più nulla da fare

	records, _ = s.Sessions()
	r := records[0]
	if r.Race == nil || r.Race.TournamentID != cup || r.Race.Type != "race" {
		t.Fatalf("gara %+v", r.Race)
	}
	var races int
	s.db.QueryRow("SELECT COUNT(*) FROM races").Scan(&races)
	if races != 1 {
		t.Fatalf("%d gare create", races)
	}
	// Le sessioni non terminate non sono mai in sospeso.
	for _, other := range records[1:] {
		if other.Race != nil {
			t.Errorf("sessione %d collegata a %+v", other.ID, other.Race)
		}
	}
}

// Se il salvataggio dei risultati non riesce, la fase resta quella dei
// risultati con l'errore in vista e si riprova da soli dopo saveRetry.
func TestLineupSaveRetry(t *testing.T) {
	packets := loadRecording(t)
	s, _ := newTestService(t, false)
	base := time.Date(2026, 10, 16, 13, 31, 38, 0, time.UTC)
	var at time.Time
	for _, p := range raceOnly(t, packets, 1) {
		at = base.Add(p.At)
		s.Handle(at, p.Data)
		if l := s.current.lineup; l != nil && l.stage == StageResults {
			break
		}
	}
	l := s.current.lineup
	if l == nil || l.stage != StageResults {
		t.Fatal("la gara non è arrivata ai risultati")
	}

	// Una tabella che manca fa fallire il salvataggio.
	s.exec("ALTER TABLE telemetry_players RENAME TO telemetry_players_off")
	expire := at.Add(lineupTimeout)
	s.updateLineup(expire)
	if l.stage != StageResults || l.saveErr == "" || !l.deadline.Equal(expire.Add(saveRetry)) {
		t.Fatalf("dopo l'errore: fase %q, errore %q, scadenza %v", l.stage, l.saveErr, l.deadline)
	}
	var snap Snapshot
	json.Unmarshal(s.snapshotJSON(expire), &snap)
	if snap.Lineup == nil || snap.Lineup.Error == "" {
		t.Fatalf("errore non mostrato: %+v", snap.Lineup)
	}

	// Al nuovo tentativo, con il database a posto, la gara si salva.
	s.exec("ALTER TABLE telemetry_players_off RENAME TO telemetry_players")
	s.updateLineup(expire.Add(saveRetry))
	if l.stage != StageSaved || l.saveErr != "" || l.raceID == nil {
		t.Fatalf("dopo il nuovo tentativo: fase %q, errore %q, gara %v", l.stage, l.saveErr, l.raceID)
	}
}

// La pagina Live dice perché la telemetria non arriva: porta occupata o formato sbagliato.
func TestTelemetryDiagnostics(t *testing.T) {
	s, _ := newTestService(t, false)
	status := func(at time.Time) TelemetryStatus {
		var snap Snapshot
		json.Unmarshal(s.snapshotJSON(at), &snap)
		if snap.Telemetry == nil {
			t.Fatal("stato della telemetria mancante")
		}
		return *snap.Telemetry
	}
	now := time.Now()

	s.listenFailed(errors.New("porta già in uso"))
	if st := status(now); st.Port != 0 || st.Error != "porta già in uso" {
		t.Fatalf("porta occupata: %+v", st)
	}
	s.listening(&net.UDPAddr{Port: 20777})
	if st := status(now); st.Port != 20777 || st.Error != "" || st.Addresses == nil {
		t.Fatalf("in ascolto: %+v", st)
	}

	old := telemetryPacket(t, telemetry.PacketEvent, 42, "SSTA")
	binary.LittleEndian.PutUint16(old, 2024)
	s.Handle(now, old)
	if st := status(now); st.BadFormat != 2024 || s.current != nil {
		t.Fatalf("formato 2024: %+v, sessione %v", st, s.current != nil)
	}
	if st := status(now.Add(badFormatFor + time.Second)); st.BadFormat != 0 {
		t.Fatalf("il formato sbagliato non arriva più ma è ancora segnalato: %+v", st)
	}
}
