package live

import "testing"

// Dalla classifica finale si salvano miglior giro, griglia, giri e soste dei
// giocatori; il risultato della gara sa chi ha fatto il giro più veloce.
func TestSessionStats(t *testing.T) {
	s, _ := recordedService(t)
	marco := insertID(t, s, "INSERT INTO players (name) VALUES ('Marco')")
	records, _ := s.Sessions()
	r := records[0]
	if err := s.SetSessionPlayers(r.ID, map[int]*int64{r.Players[0].CarIndex: &marco}); err != nil {
		t.Fatal(err)
	}

	check := func(what string) {
		t.Helper()
		var best, grid, laps, sessionBest *int64
		s.db.QueryRow("SELECT best_lap_ms, grid_position, laps FROM telemetry_players WHERE session_id = ?", r.ID).Scan(&best, &grid, &laps)
		s.db.QueryRow("SELECT best_lap_ms FROM telemetry_sessions WHERE id = ?", r.ID).Scan(&sessionBest)
		// Nella registrazione di prova HAMILTON fa 1:26.432, il giro più veloce della gara.
		if best == nil || *best != 86_432 || grid == nil || *grid == 0 || laps == nil || *laps != 5 || sessionBest == nil || *sessionBest != 86_432 {
			t.Fatalf("%s: giocatore %v %v %v, sessione %v", what, best, grid, laps, sessionBest)
		}
		var lap *int64
		var fastest bool
		s.db.QueryRow("SELECT best_lap_ms, fastest_lap FROM results WHERE race_id = ? AND player_id = ?", r.Race.ID, marco).Scan(&lap, &fastest)
		if lap == nil || *lap != 86_432 || !fastest {
			t.Fatalf("%s: risultato %v, giro veloce %v", what, lap, fastest)
		}
	}
	check("dopo la gara")

	// Una sessione registrata prima delle statistiche: si rilegge dal file, il pilota resta.
	s.exec(`UPDATE telemetry_players SET best_lap_ms = NULL, grid_position = NULL, laps = NULL WHERE session_id = ?`, r.ID)
	s.exec("UPDATE telemetry_sessions SET best_lap_ms = NULL, stats_extracted = 0 WHERE id = ?", r.ID)
	s.exec("UPDATE results SET best_lap_ms = NULL, fastest_lap = 0")
	s.backfillStats()
	check("dopo la rilettura")
	var player *int64
	s.db.QueryRow("SELECT player_id FROM telemetry_players WHERE session_id = ?", r.ID).Scan(&player)
	if player == nil || *player != marco {
		t.Fatalf("pilota cambiato dalla rilettura: %v", player)
	}
}
