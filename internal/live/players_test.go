package live

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// recordedService registra le sessioni della registrazione di test e le chiude.
func recordedService(t *testing.T) (*Service, string) {
	t.Helper()
	packets := loadRecording(t)
	s, dir := newTestService(t, true)
	base := time.Date(2026, 10, 16, 13, 31, 38, 0, time.UTC)
	for _, p := range packets {
		s.Handle(base.Add(p.At), p.Data)
	}
	s.tick(s.lastPacket.Add(sessionTimeout + time.Second))
	return s, dir
}

type raceResult struct {
	playerID int64
	position *int
	dnf      bool
}

func resultsOf(t *testing.T, s *Service, raceID int64) []raceResult {
	t.Helper()
	rows, err := s.db.Query("SELECT player_id, position, dnf FROM results WHERE race_id = ? ORDER BY player_id", raceID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []raceResult
	for rows.Next() {
		var r raceResult
		if err := rows.Scan(&r.playerID, &r.position, &r.dnf); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func insertID(t *testing.T, s *Service, query string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := s.db.QueryRow(query+" RETURNING id", args...).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestSessionPlayersAndRaceResults(t *testing.T) {
	s, dir := recordedService(t)
	simone := insertID(t, s, "INSERT INTO players (name) VALUES ('Simone')")
	marco := insertID(t, s, "INSERT INTO players (name) VALUES ('Marco')")
	tournament := insertID(t, s, "INSERT INTO tournaments (name) VALUES ('Test')")

	records, err := s.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	last := records[0]
	if !last.Analyzed || last.Cars == nil || *last.Cars != 20 || !last.ProvidesResults {
		t.Fatalf("ultima sessione %+v", last)
	}
	if len(last.Players) != 1 {
		t.Fatalf("giocatori %+v", last.Players)
	}
	p := last.Players[0]
	if p.Name != "HAMILTON" || p.Position == nil || *p.Position != 7 || p.DNF || p.PlayerID != nil {
		t.Fatalf("giocatore %+v", p)
	}

	// Finita, la gara è stata salvata in "Nessun torneo"; spostata in un torneo
	// in una nuova gara, quella rimasta vuota in "Nessun torneo" sparisce.
	if last.Race == nil {
		t.Fatal("gara non salvata in automatico")
	}
	freeRace := last.Race.ID
	raceID, err := s.LinkNewRace(last.ID, tournament, "race")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow("SELECT id FROM races WHERE id = ?", freeRace).Scan(new(int64)); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("gara vuota rimasta in \"Nessun torneo\": %v", err)
	}
	// Senza piloti associati non ci sono risultati.
	var track string
	s.db.QueryRow("SELECT track FROM races WHERE id = ?", raceID).Scan(&track)
	if track != "Italia – Monza" {
		t.Errorf("circuito della gara %q", track)
	}
	if res := resultsOf(t, s, raceID); len(res) != 0 {
		t.Fatalf("risultati senza piloti: %+v", res)
	}

	// Associato a Marco e poi corretto in Simone: i risultati seguono.
	if err := s.SetSessionPlayers(last.ID, map[int]*int64{p.CarIndex: &marco}); err != nil {
		t.Fatal(err)
	}
	if res := resultsOf(t, s, raceID); len(res) != 1 || res[0].playerID != marco || *res[0].position != 7 {
		t.Fatalf("risultati con Marco: %+v", res)
	}
	if err := s.SetSessionPlayers(last.ID, map[int]*int64{p.CarIndex: &simone}); err != nil {
		t.Fatal(err)
	}
	if res := resultsOf(t, s, raceID); len(res) != 1 || res[0].playerID != simone {
		t.Fatalf("risultati con Simone: %+v", res)
	}
	records, _ = s.Sessions()
	if r := records[0]; r.Race == nil || r.Race.ID != raceID || r.Race.TournamentName != "Test" ||
		*r.Players[0].PlayerName != "Simone" {
		t.Fatalf("sessione collegata %+v", r)
	}

	// Errori di validazione.
	var invalidErr *InvalidError
	if err := s.SetSessionPlayers(last.ID, map[int]*int64{99: &simone}); !errors.As(err, &invalidErr) {
		t.Errorf("auto inesistente: %v", err)
	}
	missing := int64(999)
	if err := s.SetSessionRace(last.ID, &missing); !errors.As(err, &invalidErr) {
		t.Errorf("gara inesistente: %v", err)
	}

	// Una sessione senza classifica si può collegare alla stessa gara senza toccare i risultati.
	if err := s.SetSessionRace(records[1].ID, &raceID); err != nil {
		t.Fatal(err)
	}
	if res := resultsOf(t, s, raceID); len(res) != 1 {
		t.Fatalf("risultati dopo il collegamento della sessione senza classifica: %+v", res)
	}

	// Rianalizzando una sessione, lo stesso nome del gioco viene associato al pilota dell'ultima volta.
	s.exec("UPDATE telemetry_sessions SET analyzed = 0 WHERE id = ?", records[2].ID)
	s.analyzePending()
	records, _ = s.Sessions()
	if pl := records[2].Players; len(pl) != 1 || pl[0].PlayerID == nil || *pl[0].PlayerID != simone {
		t.Fatalf("associazione automatica: %+v", pl)
	}

	// Una sessione finita non si può togliere dal torneo, solo spostare: i
	// risultati che venivano da lei passano alla nuova gara.
	if err := s.SetSessionRace(last.ID, nil); !errors.As(err, &invalidErr) {
		t.Fatalf("scollegamento di una sessione finita: %v", err)
	}
	other, err := s.LinkNewRace(last.ID, tournament, "")
	if err != nil {
		t.Fatal(err)
	}
	if res := resultsOf(t, s, raceID); len(res) != 0 {
		t.Fatalf("risultati della gara di prima dopo lo spostamento: %+v", res)
	}
	if res := resultsOf(t, s, other); len(res) != 1 {
		t.Fatalf("risultati della nuova gara: %+v", res)
	}
	// La gara di prima non è in "Nessun torneo": resta, anche se vuota.
	if err := s.db.QueryRow("SELECT id FROM races WHERE id = ?", raceID).Scan(new(int64)); err != nil {
		t.Fatalf("gara di prima: %v", err)
	}

	// Eliminando una registrazione spariscono riga e file, ma non i risultati della gara.
	if err := s.SetSessionRace(last.ID, &raceID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSession(last.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, last.file)); !os.IsNotExist(err) {
		t.Errorf("file non eliminato: %v", err)
	}
	if err := s.DeleteSession(last.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("seconda eliminazione: %v", err)
	}
	if res := resultsOf(t, s, raceID); len(res) != 1 {
		t.Fatalf("risultati dopo l'eliminazione: %+v", res)
	}
}

func TestDeleteSessionInProgress(t *testing.T) {
	packets := loadRecording(t)
	s, _ := newTestService(t, true)
	for _, p := range packets[:5000] {
		s.Handle(time.Now(), p.Data)
	}
	records, _ := s.Sessions()
	if err := s.DeleteSession(records[0].ID); !errors.Is(err, ErrSessionRecording) {
		t.Fatalf("eliminazione della sessione in corso: %v", err)
	}
}

func TestDeleteUnfinished(t *testing.T) {
	s, dir := recordedService(t)
	tournament := insertID(t, s, "INSERT INTO tournaments (name) VALUES ('Test')")
	race := insertID(t, s, "INSERT INTO races (tournament_id, track, type) VALUES (?, 'Monza', 'race')", tournament)

	records, _ := s.Sessions()
	// La prima (più recente) è l'unica terminata; una di quelle non terminate viene assegnata al torneo.
	if records[0].DeleteAt != nil || records[1].DeleteAt == nil {
		t.Fatalf("deleteAt: %v, %v", records[0].DeleteAt, records[1].DeleteAt)
	}
	if err := s.SetSessionRace(records[1].ID, &race); err != nil {
		t.Fatal(err)
	}

	end := time.Date(2026, 10, 16, 13, 45, 0, 0, time.UTC) // dopo la fine di tutte le sessioni
	s.deleteUnfinished(end.Add(47 * time.Hour))
	if after, _ := s.Sessions(); len(after) != 5 {
		t.Fatalf("prima delle 48 ore restano %d sessioni", len(after))
	}

	s.deleteUnfinished(end.Add(49 * time.Hour))
	after, _ := s.Sessions()
	if len(after) != 2 || after[0].ID != records[0].ID || after[1].ID != records[1].ID {
		t.Fatalf("restano %+v", after)
	}
	if files, _ := os.ReadDir(dir); len(files) != 2 {
		t.Errorf("%d file rimasti", len(files))
	}
}

// DeleteUnfinished elimina subito le sessioni non terminate, senza aspettare le 48 ore.
func TestDeleteUnfinishedNow(t *testing.T) {
	s, dir := recordedService(t)
	records, _ := s.Sessions()

	count, size, err := s.UnfinishedUsage()
	if err != nil || count != 4 || size == 0 {
		t.Fatalf("non terminate: %d, %d byte, %v", count, size, err)
	}
	deleted, freed, err := s.DeleteUnfinished()
	if err != nil || deleted != count || freed != size {
		t.Fatalf("eliminate %d, %d byte, %v", deleted, freed, err)
	}
	after, _ := s.Sessions()
	if len(after) != 1 || after[0].ID != records[0].ID {
		t.Fatalf("restano %+v", after)
	}
	if files, _ := os.ReadDir(dir); len(files) != 1 {
		t.Errorf("%d file rimasti", len(files))
	}
	if count, _, _ := s.UnfinishedUsage(); count != 0 {
		t.Errorf("ancora %d non terminate", count)
	}
}
