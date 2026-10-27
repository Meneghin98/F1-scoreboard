package api

import (
	"cmp"
	"net/http"
	"slices"
	"strconv"

	"f1-scoreboard/internal/live"
	"f1-scoreboard/internal/scoring"
	"f1-scoreboard/internal/telemetry"
)

// Statistiche di tutti i tornei, o di uno solo: carriera dei piloti, record dei
// circuiti (dai giri delle registrazioni) e testa a testa. I dati sono pochi (qualche
// centinaio di risultati): si leggono tutti e si contano in memoria.

// CareerStats sono i numeri di un pilota su tutte le gare, di tutti i tornei.
type CareerStats struct {
	PlayerID    int64    `json:"player_id"`
	Name        string   `json:"name"`
	Team        *string  `json:"team"`
	Color       string   `json:"color"`
	Races       int      `json:"races"` // Gran Premi corsi
	Sprints     int      `json:"sprints"`
	Wins        int      `json:"wins"` // vittorie nei Gran Premi
	SprintWins  int      `json:"sprintWins"`
	Podiums     int      `json:"podiums"`
	Poles       int      `json:"poles"`       // qualifiche vinte
	FastestLaps int      `json:"fastestLaps"` // giri più veloci nei Gran Premi
	DNF         int      `json:"dnf"`
	AvgFinish   *float64 `json:"avgFinish"`  // media dei piazzamenti nei Gran Premi finiti, nil se nessuno
	BestFinish  *int     `json:"bestFinish"` // miglior piazzamento in un Gran Premio
	Points      int      `json:"points"`     // somma dei punti nelle classifiche dei tornei
	Tournaments int      `json:"tournaments"`
	Titles      int      `json:"titles"` // tornei terminati da primo in classifica
}

// TrackRecord è il giro più veloce dei piloti del segnapunti su un circuito.
type TrackRecord struct {
	TrackID     int    `json:"trackId"`
	Track       string `json:"track"`
	LapMS       int64  `json:"lapMs"`
	PlayerID    int64  `json:"player_id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	SessionID   int64  `json:"sessionId"`   // registrazione del giro, per il replay
	SessionType string `json:"sessionType"` // "Gara", "Qualifica 1"…
	Date        string `json:"date"`        // UTC, inizio della sessione
}

// HeadToHead conta, per ogni coppia di piloti, le gare (GP e sprint) in cui
// uno è arrivato davanti all'altro.
type HeadToHead struct {
	Players []H2HPlayer `json:"players"` // nell'ordine della classifica di carriera
	Ahead   [][]int     `json:"ahead"`   // Ahead[i][j]: gare in cui i è arrivato davanti a j
}

type H2HPlayer struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// statResult è un risultato con quello che serve alle statistiche.
type statResult struct {
	raceID, tournamentID, playerID int64
	raceType, track                string
	date                           *string
	position                       *int
	dnf, fastest                   bool
	free, ended, active            bool // gara di "Nessun torneo"; torneo terminato; torneo in corso
	tournamentName                 string
	rules                          scoring.Rules // regole di punteggio del torneo
}

// statsData sono tutti i dati delle statistiche, letti una volta.
type statsData struct {
	players map[int64]Player
	order   []int64 // piloti in ordine di nome
	results []statResult
}

func (s *Server) loadStats() (*statsData, error) {
	d := &statsData{players: map[int64]Player{}}
	rows, err := s.db.Query("SELECT " + playerColumns + " FROM players ORDER BY name")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p Player
		if err := scanPlayer(rows, &p); err != nil {
			rows.Close()
			return nil, err
		}
		d.players[p.ID] = p
		d.order = append(d.order, p.ID)
	}
	rows.Close()

	rows, err = s.db.Query(`
		SELECT r.race_id, ra.tournament_id, r.player_id, ra.type, ra.track, ra.date, r.position, r.dnf, r.fastest_lap,
		       t.free, t.ended_at IS NOT NULL, t.started_at IS NOT NULL AND t.ended_at IS NULL, t.name,
		       t.points_mode, t.pole_points, t.fastest_lap_points, t.points_preset, t.race_points, t.sprint_points
		FROM results r JOIN races ra ON ra.id = r.race_id JOIN tournaments t ON t.id = ra.tournament_id
		ORDER BY ra.date IS NULL, ra.date, ra.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r statResult
		err := rows.Scan(&r.raceID, &r.tournamentID, &r.playerID, &r.raceType, &r.track, &r.date, &r.position, &r.dnf,
			&r.fastest, &r.free, &r.ended, &r.active, &r.tournamentName,
			&r.rules.Mode, &r.rules.PolePoints, &r.rules.FastestLapPoints, &r.rules.Preset, &r.rules.RacePoints, &r.rules.SprintPoints)
		if err != nil {
			return nil, err
		}
		d.results = append(d.results, r)
	}
	return d, rows.Err()
}

// standings calcola la classifica di ogni torneo, con le sue regole. "Nessun
// torneo" conta solo con withFree: nella somma di tutti i tornei non c'è.
func (d *statsData) standings(withFree bool) map[int64][]scoring.Standing {
	byTournament := map[int64][]scoring.Result{}
	rules := map[int64]scoring.Rules{}
	for _, r := range d.results {
		if r.free && !withFree {
			continue
		}
		p := d.players[r.playerID]
		byTournament[r.tournamentID] = append(byTournament[r.tournamentID], scoring.Result{
			RaceID: r.raceID, RaceType: r.raceType, PlayerID: r.playerID, Name: p.Name, Team: p.Team, Color: p.Color,
			Position: r.position, DNF: r.dnf, FastestLap: r.fastest,
		})
		rules[r.tournamentID] = r.rules
	}
	out := map[int64][]scoring.Standing{}
	for id, results := range byTournament {
		out[id] = scoring.Standings(results, rules[id])
	}
	return out
}

// career calcola i numeri di carriera di ogni pilota, dal migliore: punti,
// poi vittorie, poi podi.
func (d *statsData) career(standings map[int64][]scoring.Standing) []CareerStats {
	byPlayer := map[int64]*CareerStats{}
	sums := map[int64][2]int{} // somma e numero dei piazzamenti nei GP finiti
	for _, id := range d.order {
		p := d.players[id]
		byPlayer[id] = &CareerStats{PlayerID: id, Name: p.Name, Team: p.Team, Color: p.Color}
	}
	for _, r := range d.results {
		c := byPlayer[r.playerID]
		if c == nil {
			continue
		}
		finished := !r.dnf && r.position != nil
		switch r.raceType {
		case "race":
			c.Races++
			if r.fastest {
				c.FastestLaps++
			}
			if !finished {
				c.DNF++
				break
			}
			pos := *r.position
			if pos == 1 {
				c.Wins++
			}
			if pos <= 3 {
				c.Podiums++
			}
			if c.BestFinish == nil || pos < *c.BestFinish {
				c.BestFinish = &pos
			}
			s := sums[r.playerID]
			sums[r.playerID] = [2]int{s[0] + pos, s[1] + 1}
		case "sprint":
			c.Sprints++
			if finished && *r.position == 1 {
				c.SprintWins++
			}
		case "qualifying":
			if finished && *r.position == 1 {
				c.Poles++
			}
		}
	}
	for id, s := range sums {
		if c := byPlayer[id]; c != nil && s[1] > 0 {
			avg := float64(s[0]) / float64(s[1])
			c.AvgFinish = &avg
		}
	}
	ended := map[int64]bool{}
	for _, r := range d.results {
		ended[r.tournamentID] = r.ended
	}
	for tid, st := range standings {
		for _, row := range st {
			if c := byPlayer[row.PlayerID]; c != nil {
				c.Points += row.Points
				c.Tournaments++
				if row.Rank == 1 && ended[tid] {
					c.Titles++
				}
			}
		}
	}

	out := make([]CareerStats, 0, len(byPlayer))
	for _, id := range d.order {
		if c := byPlayer[id]; c.Races+c.Sprints+c.Poles > 0 {
			out = append(out, *c)
		}
	}
	slices.SortStableFunc(out, func(a, b CareerStats) int {
		return cmp.Or(cmp.Compare(b.Points, a.Points), cmp.Compare(b.Wins, a.Wins), cmp.Compare(b.Podiums, a.Podiums))
	})
	return out
}

// headToHead confronta i piloti a due a due nelle gare corse insieme.
func (d *statsData) headToHead(career []CareerStats) HeadToHead {
	h := HeadToHead{Players: []H2HPlayer{}, Ahead: [][]int{}}
	index := map[int64]int{}
	for i, c := range career {
		index[c.PlayerID] = i
		h.Players = append(h.Players, H2HPlayer{ID: c.PlayerID, Name: c.Name, Color: c.Color})
		h.Ahead = append(h.Ahead, make([]int, len(career)))
	}
	byRace := map[int64][]statResult{}
	for _, r := range d.results {
		if r.raceType == "race" || r.raceType == "sprint" {
			byRace[r.raceID] = append(byRace[r.raceID], r)
		}
	}
	for _, results := range byRace {
		for _, a := range results {
			for _, b := range results {
				i, okA := index[a.playerID]
				j, okB := index[b.playerID]
				if okA && okB && i != j && aheadOf(a, b) {
					h.Ahead[i][j]++
				}
			}
		}
	}
	return h
}

// aheadOf dice se a è arrivato davanti a b: arrivato mentre b no, o in una posizione migliore.
func aheadOf(a, b statResult) bool {
	aFinished, bFinished := !a.dnf && a.position != nil, !b.dnf && b.position != nil
	if !aFinished {
		return false
	}
	return !bFinished || *a.position < *b.position
}

// lapRecords restituisce il giro più veloce dei piloti su ogni circuito,
// nell'ordine del calendario, e i migliori giri di ogni pilota per circuito.
// Con tournamentID diverso da 0 contano solo le registrazioni delle gare di quel torneo.
func (s *Server) lapRecords(tournamentID int64) (records []TrackRecord, bestLaps map[int64][]TrackRecord, err error) {
	rows, err := s.db.Query(`
		SELECT s.track_id, tp.best_lap_ms, tp.player_id, p.name, COALESCE(p.color, ''), s.id, s.session_type, s.started_at
		FROM telemetry_players tp
		JOIN telemetry_sessions s ON s.id = tp.session_id
		JOIN players p ON p.id = tp.player_id
		WHERE tp.best_lap_ms IS NOT NULL AND s.track_id IS NOT NULL AND s.track_id >= 0
		  AND (? = 0 OR s.race_id IN (SELECT id FROM races WHERE tournament_id = ?))
		ORDER BY s.track_id, tp.best_lap_ms, s.id`, tournamentID, tournamentID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	bestLaps = map[int64][]TrackRecord{}
	seenTrack := map[int]bool{}
	seenPlayer := map[[2]int64]bool{} // circuito, pilota
	for rows.Next() {
		var r TrackRecord
		var sessionType *int
		if err := rows.Scan(&r.TrackID, &r.LapMS, &r.PlayerID, &r.Name, &r.Color, &r.SessionID, &sessionType, &r.Date); err != nil {
			return nil, nil, err
		}
		r.Track = live.RaceTrackName(telemetry.TrackID(r.TrackID))
		if sessionType != nil {
			r.SessionType = telemetry.SessionType(*sessionType).String()
		}
		// Le righe sono in ordine di tempo: la prima per circuito è il record, la prima per pilota il suo migliore.
		if !seenTrack[r.TrackID] {
			seenTrack[r.TrackID] = true
			records = append(records, r)
		}
		if key := [2]int64{int64(r.TrackID), r.PlayerID}; !seenPlayer[key] {
			seenPlayer[key] = true
			bestLaps[r.PlayerID] = append(bestLaps[r.PlayerID], r)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	order := map[int]int{}
	for i, t := range live.Tracks {
		order[t.ID] = i
	}
	byCalendar := func(a, b TrackRecord) int {
		ia, okA := order[a.TrackID]
		ib, okB := order[b.TrackID]
		if !okA {
			ia = len(order) + a.TrackID
		}
		if !okB {
			ib = len(order) + b.TrackID
		}
		return cmp.Compare(ia, ib)
	}
	slices.SortFunc(records, byCalendar)
	for id := range bestLaps {
		slices.SortFunc(bestLaps[id], byCalendar)
	}
	if records == nil {
		records = []TrackRecord{}
	}
	return records, bestLaps, nil
}

// getStats restituisce le statistiche di tutti i tornei o, con ?tournament=N, di uno solo.
func (s *Server) getStats(w http.ResponseWriter, r *http.Request) error {
	var tournamentID int64
	if q := r.URL.Query().Get("tournament"); q != "" {
		id, err := strconv.ParseInt(q, 10, 64)
		found := false
		if err == nil {
			if found, err = s.exists("SELECT 1 FROM tournaments WHERE id = ?", id); err != nil {
				return err
			}
		}
		if !found {
			return errorf(http.StatusNotFound, "torneo non trovato")
		}
		tournamentID = id
	}
	d, err := s.loadStats()
	if err != nil {
		return err
	}
	if tournamentID != 0 {
		d.results = slices.DeleteFunc(d.results, func(r statResult) bool { return r.tournamentID != tournamentID })
	}
	records, _, err := s.lapRecords(tournamentID)
	if err != nil {
		return err
	}
	// Scelto un solo torneo, anche "Nessun torneo", i punti sono quelli della sua classifica.
	career := d.career(d.standings(tournamentID != 0))
	return writeJSON(w, http.StatusOK, map[string]any{
		"career":     career,
		"records":    records,
		"headToHead": d.headToHead(career),
	})
}

// --- Profilo di un pilota ----------------------------------------------------

// PlayerTournament è il risultato di un pilota in un torneo.
type PlayerTournament struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Rank   int    `json:"rank"`
	Of     int    `json:"of"` // piloti in classifica
	Points int    `json:"points"`
	Wins   int    `json:"wins"`
	Active bool   `json:"active"` // in corso
	Ended  bool   `json:"ended"`
}

// RecentResult è un risultato recente di un pilota.
type RecentResult struct {
	RaceID         int64   `json:"raceId"`
	TournamentID   int64   `json:"tournamentId"`
	TournamentName string  `json:"tournamentName"`
	Track          string  `json:"track"`
	Type           string  `json:"type"`
	Date           *string `json:"date"`
	Position       *int    `json:"position"`
	DNF            bool    `json:"dnf"`
	FastestLap     bool    `json:"fastestLap"`
}

// Rival è il confronto diretto con un altro pilota.
type Rival struct {
	H2HPlayer
	Ahead  int `json:"ahead"`  // gare in cui il pilota è arrivato davanti al rivale
	Behind int `json:"behind"` // gare in cui è arrivato dietro
}

// getPlayerStats restituisce il profilo di un pilota: carriera, tornei,
// migliori giri per circuito, ultimi risultati e confronti diretti.
func (s *Server) getPlayerStats(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathID(r, "players", "Pilota")
	if err != nil {
		return err
	}
	d, err := s.loadStats()
	if err != nil {
		return err
	}
	records, bestLaps, err := s.lapRecords(0)
	if err != nil {
		return err
	}
	standings := d.standings(false)
	career := d.career(standings)

	profile := struct {
		Player      Player             `json:"player"`
		Career      CareerStats        `json:"career"`
		Tournaments []PlayerTournament `json:"tournaments"`
		BestLaps    []map[string]any   `json:"bestLaps"`
		Recent      []RecentResult     `json:"recent"`
		Rivals      []Rival            `json:"rivals"`
	}{Player: d.players[id], Tournaments: []PlayerTournament{}, BestLaps: []map[string]any{}, Recent: []RecentResult{}, Rivals: []Rival{}}

	p := d.players[id]
	profile.Career = CareerStats{PlayerID: id, Name: p.Name, Team: p.Team, Color: p.Color}
	for _, c := range career {
		if c.PlayerID == id {
			profile.Career = c
		}
	}

	// Tornei, dal più recente.
	names, ended, active := map[int64]string{}, map[int64]bool{}, map[int64]bool{}
	for _, r := range d.results {
		names[r.tournamentID], ended[r.tournamentID], active[r.tournamentID] = r.tournamentName, r.ended, r.active
	}
	for tid, st := range standings {
		for _, row := range st {
			if row.PlayerID == id {
				profile.Tournaments = append(profile.Tournaments, PlayerTournament{
					ID: tid, Name: names[tid], Rank: row.Rank, Of: len(st), Points: row.Points, Wins: row.Wins,
					Active: active[tid], Ended: ended[tid],
				})
			}
		}
	}
	slices.SortFunc(profile.Tournaments, func(a, b PlayerTournament) int { return cmp.Compare(b.ID, a.ID) })

	// Migliori giri per circuito, segnando i record.
	recordOf := map[int]int64{}
	for _, rec := range records {
		recordOf[rec.TrackID] = rec.LapMS
	}
	for _, lap := range bestLaps[id] {
		profile.BestLaps = append(profile.BestLaps, map[string]any{
			"trackId": lap.TrackID, "track": lap.Track, "lapMs": lap.LapMS, "sessionId": lap.SessionID,
			"sessionType": lap.SessionType, "date": lap.Date, "record": recordOf[lap.TrackID] == lap.LapMS,
		})
	}

	// Ultimi 10 risultati, dal più recente (i risultati sono in ordine di data).
	for i := len(d.results) - 1; i >= 0 && len(profile.Recent) < 10; i-- {
		r := d.results[i]
		if r.playerID == id {
			profile.Recent = append(profile.Recent, RecentResult{
				RaceID: r.raceID, TournamentID: r.tournamentID, TournamentName: r.tournamentName, Track: r.track,
				Type: r.raceType, Date: r.date, Position: r.position, DNF: r.dnf, FastestLap: r.fastest,
			})
		}
	}

	// Confronti diretti con gli altri piloti.
	h := d.headToHead(career)
	me := slices.IndexFunc(h.Players, func(p H2HPlayer) bool { return p.ID == id })
	if me >= 0 {
		for j, other := range h.Players {
			if j != me && h.Ahead[me][j]+h.Ahead[j][me] > 0 {
				profile.Rivals = append(profile.Rivals, Rival{H2HPlayer: other, Ahead: h.Ahead[me][j], Behind: h.Ahead[j][me]})
			}
		}
	}
	return writeJSON(w, http.StatusOK, profile)
}
