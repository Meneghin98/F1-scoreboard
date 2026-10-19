package live

import (
	"errors"
	"testing"
	"time"

	"f1-scoreboard/internal/recording"
)

// playRace fa correre la gara della registrazione (con SessionUID uid) a
// partire da start, fino alla chiusura della sessione: senza interventi i
// risultati vengono salvati nel torneo in corso.
func playRace(t *testing.T, s *Service, packets []recording.Packet, uid uint64, start time.Time) SessionRecord {
	t.Helper()
	for _, p := range raceOnly(t, packets, uid) {
		s.Handle(start.Add(p.At), p.Data)
	}
	s.tick(s.lastPacket.Add(finishedTimeout + time.Second))
	records, err := s.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	return records[0]
}

func countRows(t *testing.T, s *Service, query string, args ...any) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// La gara in calendario sullo stesso circuito si riempie con la registrazione,
// invece di crearne un'altra accanto.
func TestPlannedRaceIsFilled(t *testing.T) {
	packets := loadRecording(t)
	s, _ := newTestService(t, false)
	cup := insertID(t, s, "INSERT INTO tournaments (name, started_at) VALUES ('Campionato', datetime('now'))")
	// In calendario: qualifica e GP a Monza (il nome scritto a mano, senza data) e un GP a Imola.
	quali := insertID(t, s, "INSERT INTO races (tournament_id, track, type) VALUES (?, 'Italia – Monza', 'qualifying')", cup)
	monza := insertID(t, s, "INSERT INTO races (tournament_id, track, type) VALUES (?, 'italia – monza', 'race')", cup)
	insertID(t, s, "INSERT INTO races (tournament_id, track, track_id, type) VALUES (?, 'Emilia-Romagna – Imola', 27, 'race')", cup)

	base := time.Date(2026, 10, 16, 13, 31, 38, 0, time.UTC)
	r := playRace(t, s, packets, 1, base)
	if r.Race == nil || r.Race.ID != monza {
		t.Fatalf("gara %+v, attesa la %d in calendario", r.Race, monza)
	}
	if n := countRows(t, s, "SELECT COUNT(*) FROM races WHERE tournament_id = ?", cup); n != 3 {
		t.Fatalf("%d gare nel torneo, attese 3", n)
	}
	if n := countRows(t, s, "SELECT COUNT(*) FROM races WHERE id = ? AND date IS NOT NULL AND track_id = 11", monza); n != 1 {
		t.Error("la gara in calendario non ha preso data e circuito della sessione")
	}
	if n := countRows(t, s, "SELECT COUNT(*) FROM telemetry_sessions WHERE race_id = ?", quali); n != 0 {
		t.Error("la qualifica in calendario ha preso la gara")
	}

	// La gara in calendario ormai è corsa: un'altra gara a Monza ne crea una nuova.
	r = playRace(t, s, packets, 2, base.Add(time.Hour))
	if r.Race == nil || r.Race.ID == monza || r.Race.TournamentID != cup {
		t.Fatalf("seconda gara %+v", r.Race)
	}
}

// Eliminata una gara, la sua registrazione passa in "Nessun torneo" con i
// risultati; eliminata anche da lì, resta senza torneo e non ricompare.
func TestDeleteRaceMovesRecording(t *testing.T) {
	packets := loadRecording(t)
	s, _ := newTestService(t, false)
	marco := insertID(t, s, "INSERT INTO players (name) VALUES ('Marco')")
	cup := insertID(t, s, "INSERT INTO tournaments (name, started_at) VALUES ('Campionato', datetime('now'))")
	r := playRace(t, s, packets, 1, time.Date(2026, 10, 16, 13, 31, 38, 0, time.UTC))
	if err := s.SetSessionPlayers(r.ID, map[int]*int64{r.Players[0].CarIndex: &marco}); err != nil {
		t.Fatal(err)
	}

	moved, err := s.DeleteRace(r.Race.ID)
	if err != nil || moved != 1 {
		t.Fatalf("eliminazione: %d spostate, %v", moved, err)
	}
	records, _ := s.Sessions()
	got := records[0].Race
	if got == nil || got.TournamentName != "Nessun torneo" || got.Type != "race" {
		t.Fatalf("dopo l'eliminazione %+v", got)
	}
	if res := resultsOf(t, s, got.ID); len(res) != 1 || res[0].playerID != marco || *res[0].position != 7 {
		t.Fatalf("risultati %+v", res)
	}
	if n := countRows(t, s, "SELECT COUNT(*) FROM races WHERE tournament_id = ?", cup); n != 0 {
		t.Fatalf("%d gare rimaste nel torneo", n)
	}

	moved, err = s.DeleteRace(got.ID)
	if err != nil || moved != 0 {
		t.Fatalf("eliminazione da Nessun torneo: %d spostate, %v", moved, err)
	}
	s.adoptPending()
	s.recoverPending()
	if records, _ = s.Sessions(); records[0].Race != nil {
		t.Fatalf("registrazione ricomparsa in %+v", records[0].Race)
	}
	if _, err := s.DeleteRace(got.ID); !errors.Is(err, ErrRaceNotFound) {
		t.Fatalf("gara inesistente: %v", err)
	}
}

// Eliminato un torneo, le registrazioni delle sue gare passano in "Nessun torneo".
func TestDeleteTournamentMovesRecordings(t *testing.T) {
	packets := loadRecording(t)
	s, _ := newTestService(t, false)
	cup := insertID(t, s, "INSERT INTO tournaments (name, started_at) VALUES ('Campionato', datetime('now'))")
	insertID(t, s, "INSERT INTO races (tournament_id, track, type) VALUES (?, 'Miami', 'race')", cup)
	playRace(t, s, packets, 1, time.Date(2026, 10, 16, 13, 31, 38, 0, time.UTC))

	moved, err := s.DeleteTournament(cup)
	if err != nil || moved != 1 {
		t.Fatalf("eliminazione: %d spostate, %v", moved, err)
	}
	records, _ := s.Sessions()
	if r := records[0].Race; r == nil || r.TournamentName != "Nessun torneo" {
		t.Fatalf("registrazione %+v", r)
	}
	var free int64
	s.db.QueryRow("SELECT id FROM tournaments WHERE free = 1").Scan(&free)
	var invalidErr *InvalidError
	if _, err := s.DeleteTournament(free); !errors.As(err, &invalidErr) {
		t.Fatalf("eliminazione di Nessun torneo: %v", err)
	}
}

// I risultati modificati a mano non vengono riscritti dalla registrazione
// finché non si ripristinano.
func TestManualResultsAreKept(t *testing.T) {
	s, _ := recordedService(t)
	marco := insertID(t, s, "INSERT INTO players (name) VALUES ('Marco')")
	luca := insertID(t, s, "INSERT INTO players (name) VALUES ('Luca')")
	records, _ := s.Sessions()
	r := records[0]
	car := r.Players[0].CarIndex
	if err := s.SetSessionPlayers(r.ID, map[int]*int64{car: &marco}); err != nil {
		t.Fatal(err)
	}

	// Come fa l'API quando si salvano i risultati a mano: Luca vince.
	s.exec("DELETE FROM results WHERE race_id = ?", r.Race.ID)
	s.exec("INSERT INTO results (race_id, player_id, position) VALUES (?, ?, 1)", r.Race.ID, luca)
	s.exec("UPDATE races SET manual_results = 1 WHERE id = ?", r.Race.ID)

	if err := s.SetSessionPlayers(r.ID, map[int]*int64{car: &luca}); err != nil {
		t.Fatal(err)
	}
	if res := resultsOf(t, s, r.Race.ID); len(res) != 1 || res[0].playerID != luca || *res[0].position != 1 {
		t.Fatalf("risultati modificati a mano riscritti: %+v", res)
	}
	if records, _ = s.Sessions(); !records[0].Race.ManualResults {
		t.Error("la registrazione non dice che i risultati sono modificati a mano")
	}

	if err := s.ResyncRace(r.Race.ID); err != nil {
		t.Fatal(err)
	}
	if res := resultsOf(t, s, r.Race.ID); len(res) != 1 || res[0].playerID != luca || *res[0].position != 7 {
		t.Fatalf("dopo il ripristino %+v", res)
	}
	if records, _ = s.Sessions(); records[0].Race.ManualResults {
		t.Error("ancora modificati a mano dopo il ripristino")
	}

	// Una gara senza registrazione non ha nulla da ripristinare.
	other := insertID(t, s, "INSERT INTO races (tournament_id, track, type) VALUES (?, 'Monza', 'race')", r.Race.TournamentID)
	var invalidErr *InvalidError
	if err := s.ResyncRace(other); !errors.As(err, &invalidErr) {
		t.Fatalf("ripristino senza registrazione: %v", err)
	}
}

// Una gara che prende i risultati da una registrazione cambia tipo solo nello stesso gruppo.
func TestCheckRaceType(t *testing.T) {
	s, _ := recordedService(t)
	records, _ := s.Sessions()
	race := records[0].Race.ID
	var invalidErr *InvalidError
	if err := s.CheckRaceType(race, "qualifying"); !errors.As(err, &invalidErr) {
		t.Errorf("GP → qualifica: %v", err)
	}
	if err := s.CheckRaceType(race, "sprint"); err != nil {
		t.Errorf("GP → sprint: %v", err)
	}
	other := insertID(t, s, "INSERT INTO races (tournament_id, track, type) VALUES (?, 'Monza', 'race')", records[0].Race.TournamentID)
	if err := s.CheckRaceType(other, "practice"); err != nil {
		t.Errorf("gara senza registrazione: %v", err)
	}
}

// Ogni modifica fatta dal servizio cambia la versione dei dati inviata ai browser.
func TestDataVersion(t *testing.T) {
	s, _ := newTestService(t, false)
	before := s.dataVersion
	s.DataChanged()
	if s.dataVersion <= before {
		t.Fatal("la versione dei dati non cambia")
	}
	r := playRace(t, s, loadRecording(t), 1, time.Date(2026, 10, 16, 13, 31, 38, 0, time.UTC))
	if r.Race == nil || s.dataVersion <= before+1 {
		t.Fatalf("gara salvata senza cambiare versione: %d", s.dataVersion)
	}
}
