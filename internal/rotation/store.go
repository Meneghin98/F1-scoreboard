package rotation

import (
	"database/sql"
	"errors"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
)

// DB è *sql.DB o *sql.Tx.
type DB interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// Preview è quante coppie mostra State.Next: il prossimo Gran Premio e i due dopo.
const Preview = 3

// State è la turnazione del torneo in corso, come la vede il frontend.
type State struct {
	// Tournament è il torneo in corso, nil se non ce n'è uno: in "Nessun
	// torneo" la turnazione non c'è.
	Tournament *Tournament `json:"tournament"`
	Present    []int64     `json:"present"` // piloti presenti, scelti nella pagina Piloti: di default tutti
	// Enabled dice se la turnazione è attiva nel torneo in corso: si può
	// spegnere nelle impostazioni del torneo. Falso senza torneo.
	Enabled bool `json:"enabled"`
	// CanDraw dice se il sorteggio si può rifare: la turnazione è attiva e il
	// torneo non ha ancora Gran Premi.
	CanDraw bool `json:"canDraw"`
	// Next sono le prossime coppie, a partire dal prossimo Gran Premio: almeno
	// Preview, e una per ogni pista del programma. Vuoto senza torneo, con la
	// turnazione spenta o con meno di due presenti.
	Next []Pair `json:"next"`
	// Plan sono le piste del programma ancora da correre, a partire dal
	// prossimo Gran Premio, con le coppie di Next; vuoto senza torneo o senza programma.
	Plan []Planned `json:"plan"`
}

type Tournament struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Load calcola la turnazione del torneo in corso.
func Load(db DB) (State, error) {
	st := State{Present: []int64{}, Next: []Pair{}, Plan: []Planned{}}
	present, err := loadPresent(db)
	if err != nil {
		return st, err
	}
	st.Present = present

	var t Tournament
	var order string
	err = db.QueryRow(`
		SELECT id, name, rotation, rotation_order FROM tournaments
		WHERE started_at IS NOT NULL AND ended_at IS NULL AND free = 0`).Scan(&t.ID, &t.Name, &st.Enabled, &order)
	if errors.Is(err, sql.ErrNoRows) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	st.Tournament = &t
	plan, err := loadPlan(db, t.ID)
	if err != nil {
		return st, err
	}
	if st.Enabled {
		history, err := loadHistory(db, t.ID)
		if err != nil {
			return st, err
		}
		st.CanDraw = len(history) == 0
		if next := Next(present, history, parseIDs(order), max(Preview, len(plan))); next != nil {
			st.Next = next
		}
	}
	for i, track := range plan {
		p := Planned{Track: track}
		if i < len(st.Next) {
			p.Pair = &st.Next[i]
		}
		st.Plan = append(st.Plan, p)
	}
	return st, nil
}

// Si salvano gli assenti e non i presenti: così di default sono tutti presenti,
// anche i piloti aggiunti dopo.

// loadPresent legge i piloti presenti: tutti quelli del segnapunti tranne gli assenti.
func loadPresent(db DB) ([]int64, error) {
	var value string
	err := db.QueryRow("SELECT value FROM settings WHERE key = 'absent_players'").Scan(&value)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	ids, err := allPlayers(db)
	if err != nil {
		return nil, err
	}
	absent := parseIDs(value)
	present := []int64{}
	for _, id := range ids {
		if !slices.Contains(absent, id) {
			present = append(present, id)
		}
	}
	return present, nil
}

// SetPresent salva i piloti presenti: gli altri diventano assenti. Restano
// così finché non si cambiano, anche dopo un riavvio del server.
func SetPresent(db DB, present []int64) error {
	ids, err := allPlayers(db)
	if err != nil {
		return err
	}
	absent := slices.DeleteFunc(ids, func(id int64) bool { return slices.Contains(present, id) })
	_, err = db.Exec(`
		INSERT INTO settings (key, value) VALUES ('absent_players', ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`, formatIDs(absent))
	return err
}

// allPlayers restituisce gli id di tutti i piloti, in ordine.
func allPlayers(db DB) ([]int64, error) {
	rows, err := db.Query("SELECT id FROM players ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Draw sorteggia l'ordine di tutti i piloti per gli spareggi della turnazione
// del torneo (vedi Next); i piloti aggiunti dopo vengono in fondo.
func Draw(db DB, tournamentID int64) error {
	ids, err := allPlayers(db)
	if err != nil {
		return err
	}
	rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	_, err = db.Exec("UPDATE tournaments SET rotation_order = ? WHERE id = ?", formatIDs(ids), tournamentID)
	return err
}

// EnsureDrawn fa il sorteggio del torneo se non è mai stato fatto: all'avvio
// del torneo, così la prima coppia non la decide l'ordine di inserimento dei piloti.
func EnsureDrawn(db DB, tournamentID int64) error {
	var order string
	if err := db.QueryRow("SELECT rotation_order FROM tournaments WHERE id = ?", tournamentID).Scan(&order); err != nil {
		return err
	}
	if order != "" {
		return nil
	}
	return Draw(db, tournamentID)
}

// loadHistory restituisce le coppie dei Gran Premi del torneo, nell'ordine in
// cui sono stati corsi. Contano i Gran Premi con i risultati di due piloti
// esatti, anche inseriti a mano: le altre gare non si giocano in split-screen.
// Il controller viene dalla registrazione; senza, il primo pilota vale come controller 1.
func loadHistory(db DB, tournamentID int64) ([]Pair, error) {
	rows, err := db.Query(`
		SELECT r.id, res.player_id, (
			SELECT MIN(tp.controller) FROM telemetry_players tp
			JOIN telemetry_sessions ts ON ts.id = tp.session_id
			WHERE ts.race_id = r.id AND tp.player_id = res.player_id
		) AS controller
		FROM races r
		JOIN results res ON res.race_id = r.id
		WHERE r.tournament_id = ? AND r.type = 'race'
		ORDER BY r.date IS NULL, r.date,
			(SELECT MIN(ts.started_at) FROM telemetry_sessions ts WHERE ts.race_id = r.id),
			r.id, controller IS NULL, controller, res.player_id`, tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var history []Pair
	var raceID int64
	var players []int64
	flush := func() {
		if len(players) == 2 {
			history = append(history, Pair{players[0], players[1]})
		}
		players = players[:0]
	}
	for rows.Next() {
		var race, player int64
		var controller sql.NullInt64
		if err := rows.Scan(&race, &player, &controller); err != nil {
			return nil, err
		}
		if race != raceID {
			flush()
			raceID = race
		}
		players = append(players, player)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	flush()
	return history, nil
}

// parseIDs legge un elenco di id come "4,1,3"; il testo vuoto è nessun id.
func parseIDs(text string) []int64 {
	var ids []int64
	for field := range strings.SplitSeq(text, ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(field), 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

func formatIDs(ids []int64) string {
	fields := make([]string, len(ids))
	for i, id := range ids {
		fields[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(fields, ",")
}
