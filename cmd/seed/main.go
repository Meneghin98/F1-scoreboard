// Comando seed: riempie il database con dati di esempio per provare l'interfaccia.
//
//	go run ./cmd/seed
//
// Usa lo stesso DB_PATH del server e non fa nulla se nel database ci sono già dei tornei.
package main

import (
	"cmp"
	"database/sql"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"

	"f1-scoreboard/internal/live"
	"f1-scoreboard/internal/store"
)

type player struct {
	name, team string
	skill      float64 // più alto = tende ad arrivare più avanti
}

var players = []player{
	{"Simone", "Ferrari", 0.9},
	{"Luca", "McLaren", 1.0},
	{"Marco", "Red Bull Racing", 0.8},
	{"Giulia", "Mercedes", 0.85},
	{"Andrea", "Aston Martin", 0.6},
	{"Chiara", "Williams", 0.65},
	{"Davide", "Alpine", 0.5},
}

type race struct {
	track, raceType, date string
}

var tournaments = []struct {
	name  string
	races []race
}{
	{"Campionato 2025", []race{
		{"Australia – Melbourne", "race", "2025-03-16"},
		{"Cina – Shanghai", "sprint", "2025-03-22"},
		{"Cina – Shanghai", "race", "2025-03-23"},
		{"Giappone – Suzuka", "race", "2025-04-06"},
		{"Bahrain – Sakhir", "race", "2025-04-13"},
		{"Miami", "sprint", "2025-05-03"},
		{"Miami", "race", "2025-05-04"},
		{"Emilia-Romagna – Imola", "race", "2025-05-18"},
		{"Monaco", "race", "2025-05-25"},
		{"Italia – Monza", "race", ""}, // gara in programma, ancora senza risultati
	}},
	{"Torneo invernale", []race{
		{"Belgio – Spa-Francorchamps", "race", "2025-12-06"},
		{"Gran Bretagna – Silverstone", "race", "2025-12-13"},
	}},
}

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = filepath.Join("data", "scoreboard.db")
	}
	db, err := store.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Il torneo libero c'è sempre, anche nei database nuovi.
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM tournaments WHERE free = 0").Scan(&count); err != nil {
		log.Fatal(err)
	}
	if count > 0 {
		log.Fatalf("%s contiene già %d tornei: per sicurezza il seed non fa nulla", dbPath, count)
	}

	if err := seed(db); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Dati di esempio inseriti in %s\n", dbPath)
}

func seed(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Seed fisso: ogni esecuzione genera gli stessi risultati.
	rng := rand.New(rand.NewPCG(2025, 1))

	playerIDs := make([]int64, len(players))
	for i, p := range players {
		// I piloti potrebbero esistere già (il controllo riguarda solo i tornei).
		err := tx.QueryRow(`
			INSERT INTO players (name, team) VALUES (?, ?)
			ON CONFLICT (name) DO UPDATE SET name = excluded.name
			RETURNING id`, p.name, p.team).Scan(&playerIDs[i])
		if err != nil {
			return err
		}
	}

	for _, t := range tournaments {
		var tournamentID int64
		if err := tx.QueryRow("INSERT INTO tournaments (name) VALUES (?) RETURNING id", t.name).Scan(&tournamentID); err != nil {
			return err
		}
		for _, r := range t.races {
			var date any
			if r.date != "" {
				date = r.date
			}
			var raceID int64
			err := tx.QueryRow("INSERT INTO races (tournament_id, track, track_id, type, date) VALUES (?, ?, ?, ?, ?) RETURNING id",
				tournamentID, r.track, live.TrackIDByName(r.track), r.raceType, date).Scan(&raceID)
			if err != nil {
				return err
			}
			if r.date == "" {
				continue
			}
			if err := insertResults(tx, rng, raceID, playerIDs); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// insertResults genera un ordine d'arrivo plausibile: i piloti più forti tendono
// a stare davanti, ogni tanto qualcuno si ritira o salta la gara.
func insertResults(tx *sql.Tx, rng *rand.Rand, raceID int64, playerIDs []int64) error {
	type entry struct {
		id    int64
		score float64
	}
	var finishers []entry
	for i, id := range playerIDs {
		switch roll := rng.Float64(); {
		case roll < 0.06: // assente
			continue
		case roll < 0.16: // ritirato
			if _, err := tx.Exec("INSERT INTO results (race_id, player_id, position, dnf) VALUES (?, ?, NULL, 1)", raceID, id); err != nil {
				return err
			}
			continue
		}
		finishers = append(finishers, entry{id, players[i].skill + rng.Float64()*0.6})
	}

	// Ordina per punteggio decrescente.
	slices.SortFunc(finishers, func(a, b entry) int { return cmp.Compare(b.score, a.score) })
	for pos, f := range finishers {
		if _, err := tx.Exec("INSERT INTO results (race_id, player_id, position, dnf) VALUES (?, ?, ?, 0)", raceID, f.id, pos+1); err != nil {
			return err
		}
	}
	return nil
}
