package api

import (
	"cmp"
	"database/sql"
	"errors"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"f1-scoreboard/internal/live"
	"f1-scoreboard/internal/scoring"
)

type Tournament struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	CreatedAt  string  `json:"created_at"`
	Free       bool    `json:"free"`       // "Nessun torneo": raccoglie le gare fuori dai tornei
	StartedAt  *string `json:"started_at"` // UTC, nil se mai iniziato
	EndedAt    *string `json:"ended_at"`   // UTC, nil se non terminato
	Active     bool    `json:"active"`     // in corso: le gare finite vengono salvate qui
	Races      int     `json:"races"`      // gare del torneo
	Recordings int     `json:"recordings"` // registrazioni collegate alle sue gare
	// Regole di punteggio: nel JSON diventano points_mode, pole_points, fastest_lap_points,
	// points_preset, race_points e sprint_points.
	scoring.Rules
	// Leader è il primo in classifica, solo nell'elenco dei tornei: nil se non
	// ci sono ancora risultati, e sempre per "Nessun torneo".
	Leader *TournamentLeader `json:"leader,omitempty"`
}

type TournamentLeader struct {
	Name   string `json:"name"`
	Color  string `json:"color"`
	Points int    `json:"points"`
}

// tournamentColumns si usa anche dopo RETURNING: per questo le sottoquery
// indicano la tabella per nome invece che con un alias.
const tournamentColumns = "id, name, created_at, free, started_at, ended_at, started_at IS NOT NULL AND ended_at IS NULL, " +
	"(SELECT COUNT(*) FROM races WHERE races.tournament_id = tournaments.id), " +
	"(SELECT COUNT(*) FROM telemetry_sessions s JOIN races r ON r.id = s.race_id WHERE r.tournament_id = tournaments.id), " +
	"points_mode, pole_points, fastest_lap_points, points_preset, race_points, sprint_points"

// scanTournament legge una riga selezionata con tournamentColumns.
func scanTournament(row interface{ Scan(...any) error }, t *Tournament) error {
	return row.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.Free, &t.StartedAt, &t.EndedAt, &t.Active, &t.Races, &t.Recordings,
		&t.Mode, &t.PolePoints, &t.FastestLapPoints, &t.Preset, &t.RacePoints, &t.SprintPoints)
}

// rulesInput sono le regole di punteggio come arrivano nel body di un torneo:
// i campi assenti restano come sono (per un torneo nuovo, quelli di default).
type rulesInput struct {
	PointsMode       *string   `json:"points_mode"`
	PolePoints       *float64  `json:"pole_points"` // float per poter rifiutare i decimali con un messaggio chiaro
	FastestLapPoints *float64  `json:"fastest_lap_points"`
	PointsPreset     *string   `json:"points_preset"`
	RacePoints       []float64 `json:"race_points"` // assenti o null: restano come sono
	SprintPoints     []float64 `json:"sprint_points"`
}

// maxBonusPoints è il massimo dei punti per la pole e per il giro veloce:
// più di una vittoria non avrebbe senso.
const maxBonusPoints = 25

// Limiti delle tabelle dei punti per posizione: una griglia ha 22 auto.
const (
	maxTablePositions = 22
	maxPositionPoints = 100
)

// apply controlla le regole del body e le applica a rules.
func (in rulesInput) apply(rules scoring.Rules) (scoring.Rules, error) {
	if in.PointsMode != nil {
		switch *in.PointsMode {
		case scoring.ModeAbsolute, scoring.ModePlayers:
			rules.Mode = *in.PointsMode
		default:
			return rules, errorf(http.StatusBadRequest, "Modalità di punteggio \"%s\" non valida", *in.PointsMode)
		}
	}
	if in.PointsPreset != nil {
		if _, ok := scoring.Presets[*in.PointsPreset]; !ok && *in.PointsPreset != scoring.PresetCustom {
			return rules, errorf(http.StatusBadRequest, "Tabella dei punti \"%s\" non valida", *in.PointsPreset)
		}
		rules.Preset = *in.PointsPreset
	}
	bonus := []struct {
		value *float64
		dst   *int
		what  string
	}{
		{in.PolePoints, &rules.PolePoints, "la pole"},
		{in.FastestLapPoints, &rules.FastestLapPoints, "il giro veloce"},
	}
	for _, b := range bonus {
		if b.value == nil {
			continue
		}
		if v := *b.value; v < 0 || v > maxBonusPoints || v != math.Trunc(v) {
			return rules, errorf(http.StatusBadRequest, "I punti per %s devono essere un numero intero da 0 a %d", b.what, maxBonusPoints)
		}
		*b.dst = int(*b.value)
	}
	tables := []struct {
		values   []float64
		dst      *scoring.PointsTable
		raceType string
		what     string
	}{
		{in.RacePoints, &rules.RacePoints, "race", "il Gran Premio"},
		{in.SprintPoints, &rules.SprintPoints, "sprint", "la sprint"},
	}
	for _, tb := range tables {
		if tb.values == nil {
			continue
		}
		table, err := pointsTable(tb.values, tb.raceType, tb.what)
		if err != nil {
			return rules, err
		}
		*tb.dst = table
	}
	return rules, nil
}

// pointsTable controlla una tabella dei punti per posizione. Gli zeri in fondo
// si tolgono; la tabella F1 diventa nil, così resta il default.
func pointsTable(values []float64, raceType, what string) (scoring.PointsTable, error) {
	if len(values) > maxTablePositions {
		return nil, errorf(http.StatusBadRequest, "La tabella dei punti per %s ha più di %d posizioni", what, maxTablePositions)
	}
	table := make(scoring.PointsTable, len(values))
	for i, v := range values {
		if v < 0 || v > maxPositionPoints || v != math.Trunc(v) {
			return nil, errorf(http.StatusBadRequest, "I punti per %s devono essere numeri interi da 0 a %d", what, maxPositionPoints)
		}
		if i > 0 && v > values[i-1] {
			return nil, errorf(http.StatusBadRequest, "Nei punti per %s il %d° non può prendere più del %d°", what, i+1, i)
		}
		table[i] = int(v)
	}
	for len(table) > 0 && table[len(table)-1] == 0 {
		table = table[:len(table)-1]
	}
	if len(table) == 0 {
		return nil, errorf(http.StatusBadRequest, "Nei punti per %s almeno il primo deve prendere punti", what)
	}
	if slices.Equal(table, scoring.Points[raceType]) {
		return nil, nil
	}
	return table, nil
}

type Player struct {
	ID    int64   `json:"id"`
	Name  string  `json:"name"`
	Team  *string `json:"team"`
	Color string  `json:"color"` // "#rrggbb": lo stesso nei grafici, nella classifica live e sulla mappa
}

const playerColumns = "id, name, team, COALESCE(color, '')"

func scanPlayer(row interface{ Scan(...any) error }, p *Player) error {
	return row.Scan(&p.ID, &p.Name, &p.Team, &p.Color)
}

// playerColors è la tavolozza dei piloti: ognuno nuovo prende il primo colore
// non ancora usato. La migrazione 009 ha usato gli stessi, nello stesso ordine.
var playerColors = []string{
	"#e8443a", "#3987e5", "#f5a524", "#22b573", "#a970ff",
	"#ff6fae", "#2fc6d6", "#9ccc3c", "#ff8a3d", "#7f8cff",
}

// colorPattern è il formato dei colori accettati: "#rrggbb".
var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type Race struct {
	ID           int64   `json:"id"`
	TournamentID int64   `json:"tournament_id"`
	Track        string  `json:"track"`
	TrackID      *int    `json:"track_id"` // circuito del gioco, nil se il nome non è tra quelli noti
	Type         string  `json:"type"`
	Date         *string `json:"date"`
	// ManualResults è vero se i risultati sono stati modificati a mano: la
	// registrazione collegata non li riscrive finché non si ripristinano.
	ManualResults bool `json:"manual_results"`
}

const raceColumns = "id, tournament_id, track, track_id, type, date, manual_results"

func scanRace(row interface{ Scan(...any) error }, r *Race) error {
	return row.Scan(&r.ID, &r.TournamentID, &r.Track, &r.TrackID, &r.Type, &r.Date, &r.ManualResults)
}

type RaceResult struct {
	PlayerID   int64  `json:"player_id"`
	Position   *int   `json:"position"`
	DNF        bool   `json:"dnf"`
	FastestLap bool   `json:"fastest_lap"` // giro più veloce della gara, IA compresa, come salvato
	BestLapMS  *int64 `json:"best_lap_ms"` // miglior giro, solo dalle registrazioni

	// Come conta il risultato con le regole del torneo (vedi scoring.ScoreRace).
	Points           int  `json:"points"`             // compresi i bonus
	ScoredPosition   *int `json:"scored_position"`    // posizione che dà i punti: tra i soli piloti in modalità "players"
	ScoredFastestLap bool `json:"scored_fastest_lap"` // giro più veloce: tra i soli piloti in modalità "players"
	Bonus            int  `json:"bonus"`              // punti per la pole o il giro veloce, già compresi in Points
	WinBonus         int  `json:"win_bonus"`          // punti per la vittoria davanti alla IA, già compresi in Points
}

type RaceWithResults struct {
	Race
	Results []RaceResult `json:"results"`
	// Recordings sono le registrazioni collegate alla gara, dalla più recente:
	// quella con Source vero dà i risultati.
	Recordings []live.RaceRecording `json:"recordings"`
}

// --- Configurazione -------------------------------------------------------

func (s *Server) getPoints(w http.ResponseWriter, r *http.Request) error {
	return writeJSON(w, http.StatusOK, scoring.Points)
}

// listTracks restituisce i circuiti da proporre per le gare, nell'ordine del calendario.
func (s *Server) listTracks(w http.ResponseWriter, r *http.Request) error {
	return writeJSON(w, http.StatusOK, live.Tracks)
}

// --- Tornei ---------------------------------------------------------------

func (s *Server) listTournaments(w http.ResponseWriter, r *http.Request) error {
	// Prima quello in corso, poi "Nessun torneo", poi gli altri dal più recente.
	rows, err := s.db.Query("SELECT " + tournamentColumns + ` FROM tournaments
		ORDER BY started_at IS NOT NULL AND ended_at IS NULL DESC, free DESC, id DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	tournaments := []Tournament{}
	for rows.Next() {
		var t Tournament
		if err := scanTournament(rows, &t); err != nil {
			return err
		}
		tournaments = append(tournaments, t)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()

	// Il primo in classifica di ogni torneo: i tornei e i loro risultati sono pochi.
	for i := range tournaments {
		t := &tournaments[i]
		if t.Free {
			continue
		}
		results, err := s.resultsOf(t.ID)
		if err != nil {
			return err
		}
		if st := scoring.Standings(results, t.Rules); len(st) > 0 {
			t.Leader = &TournamentLeader{Name: st[0].Name, Color: st[0].Color, Points: st[0].Points}
		}
	}
	return writeJSON(w, http.StatusOK, tournaments)
}

// createTournament crea un torneo, con "start" per iniziarlo subito. Le regole
// di punteggio sono facoltative: senza, vale il regolamento F1 2025.
// Body: { "name": "...", "start": true, "points_mode": "players", "pole_points": 3, "fastest_lap_points": 1 }
func (s *Server) createTournament(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Name  string `json:"name"`
		Start bool   `json:"start"`
		rulesInput
	}
	if err := readJSON(r, &body); err != nil {
		return err
	}
	name, err := requireText(body.Name, "nome")
	if err != nil {
		return err
	}
	rules, err := body.apply(scoring.Rules{Mode: scoring.ModeAbsolute, Preset: scoring.PresetUltimateChampion})
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if body.Start {
		if err := checkNoneActive(tx); err != nil {
			return err
		}
	}
	var t Tournament
	err = scanTournament(tx.QueryRow(`
		INSERT INTO tournaments (name, started_at, points_mode, pole_points, fastest_lap_points, points_preset, race_points, sprint_points)
		VALUES (?, CASE WHEN ? THEN datetime('now') END, ?, ?, ?, ?, ?, ?)
		RETURNING `+tournamentColumns, name, body.Start, rules.Mode, rules.PolePoints, rules.FastestLapPoints,
		rules.Preset, rules.RacePoints, rules.SprintPoints), &t)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, t)
}

// updateTournament cambia nome e regole di punteggio di un torneo: i campi
// assenti restano come sono. La classifica si ricalcola con le nuove regole.
// Body come per createTournament, senza "start".
func (s *Server) updateTournament(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathID(r, "tournaments", "Torneo")
	if err != nil {
		return err
	}
	var body struct {
		Name *string `json:"name"`
		rulesInput
	}
	if err := readJSON(r, &body); err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var t Tournament
	if err := scanTournament(tx.QueryRow("SELECT "+tournamentColumns+" FROM tournaments WHERE id = ?", id), &t); err != nil {
		return err
	}
	if t.Free {
		return errorf(http.StatusBadRequest, "\"Nessun torneo\" non si può modificare: raccoglie le gare giocate fuori dai tornei")
	}
	name := t.Name
	if body.Name != nil {
		if name, err = requireText(*body.Name, "nome"); err != nil {
			return err
		}
	}
	rules, err := body.apply(t.Rules)
	if err != nil {
		return err
	}
	err = scanTournament(tx.QueryRow(`
		UPDATE tournaments
		SET name = ?, points_mode = ?, pole_points = ?, fastest_lap_points = ?, points_preset = ?, race_points = ?, sprint_points = ?
		WHERE id = ?
		RETURNING `+tournamentColumns, name, rules.Mode, rules.PolePoints, rules.FastestLapPoints,
		rules.Preset, rules.RacePoints, rules.SprintPoints, id), &t)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, t)
}

// checkNoneActive restituisce un errore se c'è già un torneo in corso.
func checkNoneActive(tx *sql.Tx) error {
	var name string
	err := tx.QueryRow("SELECT name FROM tournaments WHERE started_at IS NOT NULL AND ended_at IS NULL").Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return errorf(http.StatusConflict, "È già in corso il torneo \"%s\": terminalo prima di iniziarne un altro", name)
}

// startTournament inizia un torneo (o riprende uno terminato): da qui in poi
// le gare finite vengono salvate lì invece che in "Nessun torneo".
func (s *Server) startTournament(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathID(r, "tournaments", "Torneo")
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var t Tournament
	if err := scanTournament(tx.QueryRow("SELECT "+tournamentColumns+" FROM tournaments WHERE id = ?", id), &t); err != nil {
		return err
	}
	if t.Free {
		return errorf(http.StatusBadRequest, "\"Nessun torneo\" non si può iniziare: raccoglie le gare quando nessun torneo è in corso")
	}
	if !t.Active {
		if err := checkNoneActive(tx); err != nil {
			return err
		}
		err = scanTournament(tx.QueryRow(`
			UPDATE tournaments SET started_at = COALESCE(started_at, datetime('now')), ended_at = NULL WHERE id = ?
			RETURNING `+tournamentColumns, id), &t)
		if err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, t)
}

// endTournament termina il torneo in corso: le gare tornano in "Nessun torneo".
func (s *Server) endTournament(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathID(r, "tournaments", "Torneo")
	if err != nil {
		return err
	}
	var t Tournament
	err = scanTournament(s.db.QueryRow(`
		UPDATE tournaments SET ended_at = datetime('now') WHERE id = ? AND started_at IS NOT NULL AND ended_at IS NULL
		RETURNING `+tournamentColumns, id), &t)
	if errors.Is(err, sql.ErrNoRows) {
		return errorf(http.StatusConflict, "Il torneo non è in corso")
	}
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, t)
}

// deleteTournament elimina un torneo con le sue gare. Le registrazioni delle
// gare passano in "Nessun torneo": la risposta dice quante sono.
func (s *Server) deleteTournament(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathID(r, "tournaments", "Torneo")
	if err != nil {
		return err
	}
	free, err := s.exists("SELECT 1 FROM tournaments WHERE id = ? AND free = 1", id)
	if err != nil {
		return err
	}
	if free {
		return errorf(http.StatusConflict, "\"Nessun torneo\" non si può eliminare")
	}
	moved, err := s.live.DeleteTournament(id)
	if err := sessionError(err); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]int{"movedRecordings": moved})
}

// --- Piloti ---------------------------------------------------------------

func (s *Server) listPlayers(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.db.Query("SELECT " + playerColumns + " FROM players ORDER BY name")
	if err != nil {
		return err
	}
	defer rows.Close()

	players := []Player{}
	for rows.Next() {
		var p Player
		if err := scanPlayer(rows, &p); err != nil {
			return err
		}
		players = append(players, p)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, players)
}

// playerInput è un pilota come arriva nel body per crearlo o modificarlo.
type playerInput struct {
	name  string
	team  *string
	color *string // nil: per un pilota nuovo il primo libero, per uno esistente quello che ha
}

// readPlayer legge e controlla il body di un pilota. id è il pilota che si
// modifica (0 se è nuovo): il suo nome attuale non conta come già usato.
// Body: { "name": "...", "team": "...", "color": "#rrggbb" } (scuderia e colore facoltativi)
func (s *Server) readPlayer(r *http.Request, id int64) (playerInput, error) {
	var body struct {
		Name  string `json:"name"`
		Team  string `json:"team"`
		Color string `json:"color"`
	}
	var in playerInput
	if err := readJSON(r, &body); err != nil {
		return in, err
	}
	name, err := requireText(body.Name, "nome")
	if err != nil {
		return in, err
	}
	// La colonna name è COLLATE NOCASE: il confronto ignora maiuscole e minuscole.
	taken, err := s.exists("SELECT 1 FROM players WHERE name = ? AND id != ?", name, id)
	if err != nil {
		return in, err
	}
	if taken {
		return in, errorf(http.StatusConflict, "Esiste già un pilota chiamato \"%s\"", name)
	}
	in.name, in.team = name, optionalText(body.Team)
	if body.Color != "" {
		if !colorPattern.MatchString(body.Color) {
			return in, errorf(http.StatusBadRequest, "Colore \"%s\" non valido: serve il formato #rrggbb", body.Color)
		}
		color := strings.ToLower(body.Color)
		in.color = &color
	}
	return in, nil
}

// freeColor restituisce il primo colore della tavolozza che nessun pilota usa
// (se sono tutti usati, ricomincia dall'inizio).
func (s *Server) freeColor() (string, error) {
	rows, err := s.db.Query("SELECT color FROM players WHERE color IS NOT NULL")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	used := map[string]int{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return "", err
		}
		used[strings.ToLower(c)]++
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	// Il colore meno usato, a parità il primo della tavolozza.
	best := playerColors[0]
	for _, c := range playerColors {
		if used[c] < used[best] {
			best = c
		}
	}
	return best, nil
}

func (s *Server) createPlayer(w http.ResponseWriter, r *http.Request) error {
	in, err := s.readPlayer(r, 0)
	if err != nil {
		return err
	}
	if in.color == nil {
		color, err := s.freeColor()
		if err != nil {
			return err
		}
		in.color = &color
	}
	var p Player
	err = scanPlayer(s.db.QueryRow("INSERT INTO players (name, team, color) VALUES (?, ?, ?) RETURNING "+playerColumns,
		in.name, in.team, in.color), &p)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, p)
}

// updatePlayer cambia nome, scuderia e colore di un pilota. Body come per createPlayer.
func (s *Server) updatePlayer(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathID(r, "players", "Pilota")
	if err != nil {
		return err
	}
	in, err := s.readPlayer(r, id)
	if err != nil {
		return err
	}
	var p Player
	err = scanPlayer(s.db.QueryRow("UPDATE players SET name = ?, team = ?, color = COALESCE(?, color) WHERE id = ? RETURNING "+playerColumns,
		in.name, in.team, in.color, id), &p)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, p)
}

func (s *Server) deletePlayer(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathID(r, "players", "Pilota")
	if err != nil {
		return err
	}
	hasResults, err := s.exists("SELECT 1 FROM results WHERE player_id = ?", id)
	if err != nil {
		return err
	}
	if hasResults {
		return errorf(http.StatusConflict, "Il pilota ha già dei risultati registrati e non può essere eliminato")
	}
	if _, err := s.db.Exec("DELETE FROM players WHERE id = ?", id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// --- Gare -----------------------------------------------------------------

func (s *Server) listRaces(w http.ResponseWriter, r *http.Request) error {
	tournamentID, err := s.pathID(r, "tournaments", "Torneo")
	if err != nil {
		return err
	}
	rules, err := s.rulesOf(tournamentID)
	if err != nil {
		return err
	}
	races, err := s.racesOf(tournamentID)
	if err != nil {
		return err
	}
	recordings, err := s.live.TournamentRecordings(tournamentID)
	if err != nil {
		return err
	}

	rows, err := s.db.Query(`
		SELECT r.race_id, r.player_id, r.position, r.dnf, r.fastest_lap, r.best_lap_ms
		FROM results r JOIN races ra ON ra.id = r.race_id
		WHERE ra.tournament_id = ?`, tournamentID)
	if err != nil {
		return err
	}
	defer rows.Close()

	resultsByRace := map[int64][]RaceResult{}
	for rows.Next() {
		var raceID int64
		var res RaceResult
		if err := rows.Scan(&raceID, &res.PlayerID, &res.Position, &res.DNF, &res.FastestLap, &res.BestLapMS); err != nil {
			return err
		}
		resultsByRace[raceID] = append(resultsByRace[raceID], res)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	out := make([]RaceWithResults, 0, len(races))
	for _, race := range races {
		results := resultsByRace[race.ID]
		if results == nil {
			results = []RaceResult{}
		}
		// I punti di ognuno dipendono anche dagli altri risultati della gara.
		input := make([]scoring.Result, len(results))
		for i, res := range results {
			input[i] = scoring.Result{RaceType: race.Type, Position: res.Position, DNF: res.DNF, FastestLap: res.FastestLap, BestLapMS: res.BestLapMS}
		}
		for i, sc := range scoring.ScoreRace(input, rules) {
			res := &results[i]
			res.Points, res.ScoredPosition, res.ScoredFastestLap, res.Bonus, res.WinBonus = sc.Points, sc.Position, sc.FastestLap, sc.Bonus, sc.WinBonus
		}
		// Ordine di arrivo, con i non classificati in fondo.
		slices.SortStableFunc(results, func(a, b RaceResult) int {
			return cmp.Compare(positionOrLast(a.Position), positionOrLast(b.Position))
		})
		recs := recordings[race.ID]
		if recs == nil {
			recs = []live.RaceRecording{}
		}
		out = append(out, RaceWithResults{Race: race, Results: results, Recordings: recs})
	}
	return writeJSON(w, http.StatusOK, out)
}

func positionOrLast(position *int) int {
	if position == nil {
		return math.MaxInt
	}
	return *position
}

// racesOf restituisce le gare di un torneo in ordine di data; quelle senza
// data (ancora da programmare) vanno in fondo, nell'ordine di inserimento.
func (s *Server) racesOf(tournamentID int64) ([]Race, error) {
	rows, err := s.db.Query(
		"SELECT "+raceColumns+" FROM races WHERE tournament_id = ? ORDER BY date IS NULL, date, id", tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	races := []Race{}
	for rows.Next() {
		var race Race
		if err := scanRace(rows, &race); err != nil {
			return nil, err
		}
		races = append(races, race)
	}
	return races, rows.Err()
}

// raceBody è il body per creare o modificare una gara.
type raceBody struct {
	Track string `json:"track"`
	Type  string `json:"type"`
	Date  string `json:"date"`
}

// readRace legge e controlla il body di una gara. Il tipo vuoto vale "race".
func readRace(r *http.Request) (track, raceType string, date *string, err error) {
	var body raceBody
	if err := readJSON(r, &body); err != nil {
		return "", "", nil, err
	}
	if track, err = requireText(body.Track, "circuito"); err != nil {
		return "", "", nil, err
	}
	switch body.Type {
	case "":
		raceType = "race"
	case "race", "sprint", "qualifying", "practice":
		raceType = body.Type
	default:
		return "", "", nil, errorf(http.StatusBadRequest, "Tipo di gara \"%s\" non valido", body.Type)
	}
	return track, raceType, optionalText(body.Date), nil
}

func (s *Server) createRace(w http.ResponseWriter, r *http.Request) error {
	tournamentID, err := s.pathID(r, "tournaments", "Torneo")
	if err != nil {
		return err
	}
	track, raceType, date, err := readRace(r)
	if err != nil {
		return err
	}
	var race Race
	err = scanRace(s.db.QueryRow(`
		INSERT INTO races (tournament_id, track, track_id, type, date) VALUES (?, ?, ?, ?, ?)
		RETURNING `+raceColumns,
		tournamentID, track, live.TrackIDByName(track), raceType, date,
	), &race)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, race)
}

// updateRace cambia circuito, tipo e data di una gara. Body come per createRace.
func (s *Server) updateRace(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathID(r, "races", "Gara")
	if err != nil {
		return err
	}
	track, raceType, date, err := readRace(r)
	if err != nil {
		return err
	}
	if err := sessionError(s.live.CheckRaceType(id, raceType)); err != nil {
		return err
	}
	var race Race
	err = scanRace(s.db.QueryRow(`
		UPDATE races SET track = ?, track_id = ?, type = ?, date = ? WHERE id = ?
		RETURNING `+raceColumns,
		track, live.TrackIDByName(track), raceType, date, id,
	), &race)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, race)
}

// deleteRace elimina una gara con i suoi risultati. Le sue registrazioni
// passano in "Nessun torneo" (o restano senza torneo, se la gara era già lì):
// la risposta dice quante sono state spostate.
func (s *Server) deleteRace(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathID(r, "races", "Gara")
	if err != nil {
		return err
	}
	moved, err := s.live.DeleteRace(id)
	if errors.Is(err, live.ErrRaceNotFound) {
		return errorf(http.StatusNotFound, "Gara non trovata")
	}
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]int{"movedRecordings": moved})
}

// saveResults sostituisce in blocco i risultati di una gara. Se la gara li
// prendeva da una registrazione, da qui restano quelli inseriti a mano
// (vedi resyncRace per tornare a quelli della registrazione). I migliori giri
// presi dalla registrazione restano a chi aveva già un risultato.
// Body: { "results": [{ "player_id": 1, "position": 3, "dnf": false, "fastest_lap": true }] }
func (s *Server) saveResults(w http.ResponseWriter, r *http.Request) error {
	raceID, err := s.pathID(r, "races", "Gara")
	if err != nil {
		return err
	}
	var body struct {
		Results *[]struct {
			PlayerID   int64    `json:"player_id"`
			Position   *float64 `json:"position"` // float per poter rifiutare i decimali con un messaggio chiaro
			DNF        bool     `json:"dnf"`
			FastestLap bool     `json:"fastest_lap"`
		} `json:"results"`
	}
	if err := readJSON(r, &body); err != nil {
		return err
	}
	if body.Results == nil {
		return errorf(http.StatusBadRequest, "Formato risultati non valido")
	}

	type row struct {
		playerID int64
		position *int
		dnf      bool
		fastest  bool
	}
	rows := make([]row, 0, len(*body.Results))
	seenPlayers := map[int64]bool{}
	seenPositions := map[int]bool{}
	fastest := 0

	for _, in := range *body.Results {
		if in.FastestLap {
			if fastest++; fastest > 1 {
				return errorf(http.StatusBadRequest, "Il giro più veloce può averlo un solo pilota")
			}
		}
		found, err := s.exists("SELECT 1 FROM players WHERE id = ?", in.PlayerID)
		if err != nil {
			return err
		}
		if !found {
			return errorf(http.StatusBadRequest, "Pilota %d inesistente", in.PlayerID)
		}
		if seenPlayers[in.PlayerID] {
			return errorf(http.StatusBadRequest, "Pilota ripetuto nei risultati")
		}
		seenPlayers[in.PlayerID] = true

		res := row{playerID: in.PlayerID, dnf: in.DNF, fastest: in.FastestLap}
		if !in.DNF {
			if in.Position == nil || *in.Position < 1 || *in.Position != math.Trunc(*in.Position) {
				return errorf(http.StatusBadRequest, "Le posizioni devono essere numeri interi maggiori di zero")
			}
			position := int(*in.Position)
			if seenPositions[position] {
				return errorf(http.StatusBadRequest, "Posizione %d assegnata più volte", position)
			}
			seenPositions[position] = true
			res.position = &position
		}
		rows = append(rows, res)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // non fa nulla se la transazione è già stata confermata

	// Il body non porta i migliori giri: si tengono quelli presi dalla registrazione.
	bestLaps := map[int64]*int64{}
	lapRows, err := tx.Query("SELECT player_id, best_lap_ms FROM results WHERE race_id = ?", raceID)
	if err != nil {
		return err
	}
	for lapRows.Next() {
		var id int64
		var lap *int64
		if err := lapRows.Scan(&id, &lap); err != nil {
			lapRows.Close()
			return err
		}
		bestLaps[id] = lap
	}
	lapRows.Close()

	if _, err := tx.Exec("DELETE FROM results WHERE race_id = ?", raceID); err != nil {
		return err
	}
	for _, res := range rows {
		_, err := tx.Exec("INSERT INTO results (race_id, player_id, position, dnf, fastest_lap, best_lap_ms) VALUES (?, ?, ?, ?, ?, ?)",
			raceID, res.playerID, res.position, res.dnf, res.fastest, bestLaps[res.playerID])
		if err != nil {
			return err
		}
	}
	if _, err := tx.Exec("UPDATE races SET manual_results = 1 WHERE id = ?", raceID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// resyncRace riprende i risultati della gara dalla sua registrazione,
// annullando le modifiche fatte a mano.
func (s *Server) resyncRace(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathID(r, "races", "Gara")
	if err != nil {
		return err
	}
	if err := sessionError(s.live.ResyncRace(id)); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// --- Classifica -----------------------------------------------------------

func (s *Server) getStandings(w http.ResponseWriter, r *http.Request) error {
	tournamentID, err := s.pathID(r, "tournaments", "Torneo")
	if err != nil {
		return err
	}

	var tournament Tournament
	err = scanTournament(s.db.QueryRow("SELECT "+tournamentColumns+" FROM tournaments WHERE id = ?", tournamentID), &tournament)
	if err != nil {
		return err
	}
	races, err := s.racesOf(tournamentID)
	if err != nil {
		return err
	}
	// In classifica solo le gare che danno punti: niente prove libere, e le
	// qualifiche solo se il torneo dà punti per la pole.
	races = slices.DeleteFunc(races, func(r Race) bool { return !tournament.Rules.Counts(r.Type) })
	results, err := s.resultsOf(tournamentID)
	if err != nil {
		return err
	}
	recordings, err := s.live.TournamentRecordings(tournamentID)
	if err != nil {
		return err
	}
	out := make([]StandingRace, len(races))
	for i, race := range races {
		out[i] = StandingRace{Race: race}
		// Come nella pagina Gare: la registrazione dei risultati, o la più recente.
		if recs := recordings[race.ID]; len(recs) > 0 {
			rec := recs[0]
			if j := slices.IndexFunc(recs, func(r live.RaceRecording) bool { return r.Source }); j >= 0 {
				rec = recs[j]
			}
			out[i].Restarts, out[i].Flashbacks = rec.Restarts, rec.Flashbacks
		}
	}

	return writeJSON(w, http.StatusOK, map[string]any{
		"tournament": tournament,
		"races":      out,
		"standings":  scoring.Standings(results, tournament.Rules),
	})
}

// StandingRace è una gara in classifica, con riavvii e flashback della sua
// registrazione (nil se non ne ha o non sono ancora contati).
type StandingRace struct {
	Race
	Restarts   *int `json:"restarts"`
	Flashbacks *int `json:"flashbacks"`
}

// rulesOf restituisce le regole di punteggio di un torneo.
func (s *Server) rulesOf(tournamentID int64) (scoring.Rules, error) {
	var rules scoring.Rules
	err := s.db.QueryRow(`
		SELECT points_mode, pole_points, fastest_lap_points, points_preset, race_points, sprint_points
		FROM tournaments WHERE id = ?`, tournamentID).
		Scan(&rules.Mode, &rules.PolePoints, &rules.FastestLapPoints, &rules.Preset, &rules.RacePoints, &rules.SprintPoints)
	return rules, err
}

// resultsOf restituisce tutti i risultati di un torneo, con i dati dei piloti.
func (s *Server) resultsOf(tournamentID int64) ([]scoring.Result, error) {
	rows, err := s.db.Query(`
		SELECT r.race_id, ra.type, r.player_id, p.name, p.team, COALESCE(p.color, ''), r.position, r.dnf, r.fastest_lap, r.best_lap_ms
		FROM results r
		JOIN races ra ON ra.id = r.race_id
		JOIN players p ON p.id = r.player_id
		WHERE ra.tournament_id = ?`, tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []scoring.Result
	for rows.Next() {
		var res scoring.Result
		err := rows.Scan(&res.RaceID, &res.RaceType, &res.PlayerID, &res.Name, &res.Team, &res.Color, &res.Position, &res.DNF,
			&res.FastestLap, &res.BestLapMS)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, rows.Err()
}
