package api

import (
	"cmp"
	"math"
	"net/http"
	"slices"

	"f1-scoreboard/internal/scoring"
)

type Tournament struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type Player struct {
	ID   int64   `json:"id"`
	Name string  `json:"name"`
	Team *string `json:"team"`
}

type Race struct {
	ID           int64   `json:"id"`
	TournamentID int64   `json:"tournament_id"`
	Track        string  `json:"track"`
	Type         string  `json:"type"`
	Date         *string `json:"date"`
}

type RaceResult struct {
	PlayerID int64 `json:"player_id"`
	Position *int  `json:"position"`
	DNF      bool  `json:"dnf"`
	Points   int   `json:"points"`
}

type RaceWithResults struct {
	Race
	Results []RaceResult `json:"results"`
}

// --- Configurazione -------------------------------------------------------

func (s *Server) getPoints(w http.ResponseWriter, r *http.Request) error {
	return writeJSON(w, http.StatusOK, scoring.Points)
}

// --- Tornei ---------------------------------------------------------------

func (s *Server) listTournaments(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.db.Query("SELECT id, name, created_at FROM tournaments ORDER BY id DESC")
	if err != nil {
		return err
	}
	defer rows.Close()

	tournaments := []Tournament{}
	for rows.Next() {
		var t Tournament
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return err
		}
		tournaments = append(tournaments, t)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, tournaments)
}

func (s *Server) createTournament(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		return err
	}
	name, err := requireText(body.Name, "nome")
	if err != nil {
		return err
	}

	var t Tournament
	err = s.db.QueryRow("INSERT INTO tournaments (name) VALUES (?) RETURNING id, name, created_at", name).
		Scan(&t.ID, &t.Name, &t.CreatedAt)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, t)
}

func (s *Server) deleteTournament(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathID(r, "tournaments", "Torneo")
	if err != nil {
		return err
	}
	if _, err := s.db.Exec("DELETE FROM tournaments WHERE id = ?", id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// --- Piloti ---------------------------------------------------------------

func (s *Server) listPlayers(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.db.Query("SELECT id, name, team FROM players ORDER BY name")
	if err != nil {
		return err
	}
	defer rows.Close()

	players := []Player{}
	for rows.Next() {
		var p Player
		if err := rows.Scan(&p.ID, &p.Name, &p.Team); err != nil {
			return err
		}
		players = append(players, p)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, players)
}

func (s *Server) createPlayer(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Name string `json:"name"`
		Team string `json:"team"`
	}
	if err := readJSON(r, &body); err != nil {
		return err
	}
	name, err := requireText(body.Name, "nome")
	if err != nil {
		return err
	}

	// La colonna name è COLLATE NOCASE: il confronto ignora maiuscole e minuscole.
	taken, err := s.exists("SELECT 1 FROM players WHERE name = ?", name)
	if err != nil {
		return err
	}
	if taken {
		return errorf(http.StatusConflict, "Esiste già un pilota chiamato \"%s\"", name)
	}

	var p Player
	err = s.db.QueryRow("INSERT INTO players (name, team) VALUES (?, ?) RETURNING id, name, team", name, optionalText(body.Team)).
		Scan(&p.ID, &p.Name, &p.Team)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, p)
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
	races, err := s.racesOf(tournamentID)
	if err != nil {
		return err
	}

	rows, err := s.db.Query(`
		SELECT r.race_id, r.player_id, r.position, r.dnf
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
		if err := rows.Scan(&raceID, &res.PlayerID, &res.Position, &res.DNF); err != nil {
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
		for i := range results {
			results[i].Points = scoring.PointsFor(race.Type, results[i].Position, results[i].DNF)
		}
		// Ordine di arrivo, con i non classificati in fondo.
		slices.SortStableFunc(results, func(a, b RaceResult) int {
			return cmp.Compare(positionOrLast(a.Position), positionOrLast(b.Position))
		})
		out = append(out, RaceWithResults{Race: race, Results: results})
	}
	return writeJSON(w, http.StatusOK, out)
}

func positionOrLast(position *int) int {
	if position == nil {
		return math.MaxInt
	}
	return *position
}

// racesOf restituisce le gare di un torneo in ordine di inserimento.
func (s *Server) racesOf(tournamentID int64) ([]Race, error) {
	rows, err := s.db.Query(
		"SELECT id, tournament_id, track, type, date FROM races WHERE tournament_id = ? ORDER BY id", tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	races := []Race{}
	for rows.Next() {
		var race Race
		if err := rows.Scan(&race.ID, &race.TournamentID, &race.Track, &race.Type, &race.Date); err != nil {
			return nil, err
		}
		races = append(races, race)
	}
	return races, rows.Err()
}

func (s *Server) createRace(w http.ResponseWriter, r *http.Request) error {
	tournamentID, err := s.pathID(r, "tournaments", "Torneo")
	if err != nil {
		return err
	}
	var body struct {
		Track string `json:"track"`
		Type  string `json:"type"`
		Date  string `json:"date"`
	}
	if err := readJSON(r, &body); err != nil {
		return err
	}
	track, err := requireText(body.Track, "circuito")
	if err != nil {
		return err
	}
	raceType := "race"
	if body.Type == "sprint" {
		raceType = "sprint"
	}

	var race Race
	err = s.db.QueryRow(`
		INSERT INTO races (tournament_id, track, type, date) VALUES (?, ?, ?, ?)
		RETURNING id, tournament_id, track, type, date`,
		tournamentID, track, raceType, optionalText(body.Date),
	).Scan(&race.ID, &race.TournamentID, &race.Track, &race.Type, &race.Date)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, race)
}

func (s *Server) deleteRace(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathID(r, "races", "Gara")
	if err != nil {
		return err
	}
	if _, err := s.db.Exec("DELETE FROM races WHERE id = ?", id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// saveResults sostituisce in blocco i risultati di una gara.
// Body: { "results": [{ "player_id": 1, "position": 3, "dnf": false }] }
func (s *Server) saveResults(w http.ResponseWriter, r *http.Request) error {
	raceID, err := s.pathID(r, "races", "Gara")
	if err != nil {
		return err
	}
	var body struct {
		Results *[]struct {
			PlayerID int64    `json:"player_id"`
			Position *float64 `json:"position"` // float per poter rifiutare i decimali con un messaggio chiaro
			DNF      bool     `json:"dnf"`
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
	}
	rows := make([]row, 0, len(*body.Results))
	seenPlayers := map[int64]bool{}
	seenPositions := map[int]bool{}

	for _, in := range *body.Results {
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

		res := row{playerID: in.PlayerID, dnf: in.DNF}
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

	if _, err := tx.Exec("DELETE FROM results WHERE race_id = ?", raceID); err != nil {
		return err
	}
	for _, res := range rows {
		_, err := tx.Exec("INSERT INTO results (race_id, player_id, position, dnf) VALUES (?, ?, ?, ?)",
			raceID, res.playerID, res.position, res.dnf)
		if err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
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
	err = s.db.QueryRow("SELECT id, name, created_at FROM tournaments WHERE id = ?", tournamentID).
		Scan(&tournament.ID, &tournament.Name, &tournament.CreatedAt)
	if err != nil {
		return err
	}
	races, err := s.racesOf(tournamentID)
	if err != nil {
		return err
	}
	results, err := s.resultsOf(tournamentID)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, map[string]any{
		"tournament": tournament,
		"races":      races,
		"standings":  scoring.Standings(results),
	})
}

// resultsOf restituisce tutti i risultati di un torneo, con i dati dei piloti.
func (s *Server) resultsOf(tournamentID int64) ([]scoring.Result, error) {
	rows, err := s.db.Query(`
		SELECT r.race_id, ra.type, r.player_id, p.name, p.team, r.position, r.dnf
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
		if err := rows.Scan(&res.RaceID, &res.RaceType, &res.PlayerID, &res.Name, &res.Team, &res.Position, &res.DNF); err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, rows.Err()
}
