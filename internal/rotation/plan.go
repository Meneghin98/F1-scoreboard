package rotation

import (
	"database/sql"
	"errors"
	"math/rand/v2"
	"slices"
)

// Il programma sono le piste suggerite per i prossimi Gran Premi del torneo in
// corso, scelte a caso tra quelle non escluse. È solo un suggerimento: non
// crea gare, e avanza di una pista per ogni Gran Premio corso, qualunque pista
// si corra davvero, come la turnazione. Così la pista k del programma va con
// la k-esima coppia di Next.

// Planned è un Gran Premio del programma.
type Planned struct {
	Track int `json:"track"` // circuito (telemetry.TrackID, vedi live.Tracks)
	// Pair è chi corre secondo la turnazione, nil se è spenta o se ci sono
	// meno di due presenti.
	Pair *Pair `json:"pair"`
}

// PickTracks sceglie n piste diverse tra available, in ordine casuale. Vengono
// prima quelle corse meno volte nel torneo (raced conta i Gran Premi per
// pista), così un programma nuovo non ripropone le piste già corse finché ce ne
// sono altre. Se n supera le piste disponibili le restituisce tutte.
func PickTracks(available []int, raced map[int]int, n int) []int {
	tracks := slices.Compact(slices.Sorted(slices.Values(available)))
	shuffle(tracks)
	slices.SortStableFunc(tracks, func(a, b int) int { return raced[a] - raced[b] })
	tracks = tracks[:max(0, min(n, len(tracks)))]
	shuffle(tracks) // altrimenti in fondo al programma finirebbero sempre le piste già corse
	return tracks
}

func shuffle(s []int) {
	rand.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
}

// GeneratePlan genera il programma del torneo: n piste tra available, senza
// quelle escluse (vedi PickTracks). Sostituisce il programma precedente.
func GeneratePlan(db DB, tournamentID int64, available []int, n int) error {
	excluded, err := LoadExcluded(db)
	if err != nil {
		return err
	}
	available = slices.DeleteFunc(slices.Clone(available), func(id int) bool { return slices.Contains(excluded, id) })
	raced := map[int]int{}
	rows, err := db.Query(`
		SELECT track_id, COUNT(*) FROM races r
		WHERE tournament_id = ? AND type = 'race' AND track_id IS NOT NULL
		  AND EXISTS (SELECT 1 FROM results WHERE race_id = r.id)
		GROUP BY track_id`, tournamentID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var track, count int
		if err := rows.Scan(&track, &count); err != nil {
			return err
		}
		raced[track] = count
	}
	if err := rows.Err(); err != nil {
		return err
	}
	done, err := grandsPrix(db, tournamentID)
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE tournaments SET track_plan = ?, track_plan_from = ? WHERE id = ?",
		formatTracks(PickTracks(available, raced, n)), done, tournamentID)
	return err
}

// ClearPlan cancella il programma del torneo.
func ClearPlan(db DB, tournamentID int64) error {
	_, err := db.Exec("UPDATE tournaments SET track_plan = '', track_plan_from = 0 WHERE id = ?", tournamentID)
	return err
}

// loadPlan restituisce le piste del programma ancora da correre, a partire
// dal prossimo Gran Premio.
func loadPlan(db DB, tournamentID int64) ([]int, error) {
	var plan string
	var from int
	err := db.QueryRow("SELECT track_plan, track_plan_from FROM tournaments WHERE id = ?", tournamentID).Scan(&plan, &from)
	if err != nil {
		return nil, err
	}
	tracks := parseTracks(plan)
	if len(tracks) == 0 {
		return tracks, nil
	}
	done, err := grandsPrix(db, tournamentID)
	if err != nil {
		return nil, err
	}
	// Se nel frattempo è stato eliminato un Gran Premio, il programma riparte dall'inizio.
	return tracks[min(max(0, done-from), len(tracks)):], nil
}

// grandsPrix conta i Gran Premi corsi nel torneo, cioè con i risultati.
func grandsPrix(db DB, tournamentID int64) (int, error) {
	var n int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM races r
		WHERE tournament_id = ? AND type = 'race' AND EXISTS (SELECT 1 FROM results WHERE race_id = r.id)`,
		tournamentID).Scan(&n)
	return n, err
}

// LoadExcluded restituisce le piste da non suggerire, scelte nella pagina Piste.
func LoadExcluded(db DB) ([]int, error) {
	var value string
	err := db.QueryRow("SELECT value FROM settings WHERE key = 'excluded_tracks'").Scan(&value)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return parseTracks(value), nil
}

// SetExcluded salva le piste da non suggerire, per tutti i tornei.
func SetExcluded(db DB, tracks []int) error {
	_, err := db.Exec(`
		INSERT INTO settings (key, value) VALUES ('excluded_tracks', ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`,
		formatTracks(slices.Compact(slices.Sorted(slices.Values(tracks)))))
	return err
}

// parseTracks legge un elenco di piste come "11,13,5"; il testo vuoto è nessuna pista.
func parseTracks(text string) []int {
	tracks := []int{}
	for _, id := range parseIDs(text) {
		tracks = append(tracks, int(id))
	}
	return tracks
}

func formatTracks(tracks []int) string {
	ids := make([]int64, len(tracks))
	for i, id := range tracks {
		ids[i] = int64(id)
	}
	return formatIDs(ids)
}
