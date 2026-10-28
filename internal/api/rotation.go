package api

import (
	"database/sql"
	"math"
	"net/http"
	"slices"

	"f1-scoreboard/internal/live"
	"f1-scoreboard/internal/rotation"
)

// getRotation restituisce la turnazione del torneo in corso: presenti e prossime coppie.
func (s *Server) getRotation(w http.ResponseWriter, r *http.Request) error {
	st, err := rotation.Load(s.db)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, st)
}

// setPresent sceglie i piloti presenti e restituisce la turnazione aggiornata.
// Body: { "players": [1, 4, 5] }
func (s *Server) setPresent(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Players []int64 `json:"players"`
	}
	if err := readJSON(r, &body); err != nil {
		return err
	}
	ids := slices.Compact(slices.Sorted(slices.Values(body.Players)))
	for _, id := range ids {
		var exists bool
		if err := s.db.QueryRow("SELECT EXISTS (SELECT 1 FROM players WHERE id = ?)", id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return errorf(http.StatusBadRequest, "Pilota %d non trovato", id)
		}
	}
	if err := rotation.SetPresent(s.db, ids); err != nil {
		return err
	}
	return s.getRotation(w, r)
}

// drawRotation rifà il sorteggio della turnazione del torneo in corso, finché
// non ha Gran Premi: decide la prima coppia. Restituisce la turnazione aggiornata.
func (s *Server) drawRotation(w http.ResponseWriter, r *http.Request) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	st, err := rotation.Load(tx)
	if err != nil {
		return err
	}
	if st.Tournament == nil {
		return errorf(http.StatusConflict, "Nessun torneo in corso: la turnazione c'è solo nei tornei")
	}
	if !st.Enabled {
		return errorf(http.StatusConflict, "La turnazione è spenta in questo torneo: si attiva nelle impostazioni del torneo")
	}
	if !st.CanDraw {
		return errorf(http.StatusConflict, "Il torneo ha già dei Gran Premi: le coppie ormai si calcolano dalle gare corse")
	}
	if err := rotation.Draw(tx, st.Tournament.ID); err != nil {
		return err
	}
	if st, err = rotation.Load(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, st)
}

// generatePlan genera il programma del torneo in corso: "races" piste diverse
// tra quelle non escluse, al posto del programma precedente. Restituisce la
// turnazione aggiornata. Body: { "races": 6 }
func (s *Server) generatePlan(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Races float64 `json:"races"` // float per poter rifiutare i decimali con un messaggio chiaro
	}
	if err := readJSON(r, &body); err != nil {
		return err
	}
	return s.changePlan(w, func(tx *sql.Tx, tournamentID int64) error {
		excluded, err := rotation.LoadExcluded(tx)
		if err != nil {
			return err
		}
		var available []int
		for _, t := range live.Tracks {
			if !slices.Contains(excluded, t.ID) {
				available = append(available, t.ID)
			}
		}
		if len(available) == 0 {
			return errorf(http.StatusBadRequest, "Tutte le piste sono escluse: scegline almeno una da giocare")
		}
		if n := body.Races; n < 1 || n > float64(len(available)) || n != math.Trunc(n) {
			return errorf(http.StatusBadRequest, "Il numero di gare deve essere un intero da 1 a %d, le piste da giocare", len(available))
		}
		return rotation.GeneratePlan(tx, tournamentID, available, int(body.Races))
	})
}

// clearPlan cancella il programma del torneo in corso e restituisce la turnazione aggiornata.
func (s *Server) clearPlan(w http.ResponseWriter, r *http.Request) error {
	return s.changePlan(w, func(tx *sql.Tx, tournamentID int64) error {
		return rotation.ClearPlan(tx, tournamentID)
	})
}

// changePlan applica change al programma del torneo in corso e risponde con la turnazione aggiornata.
func (s *Server) changePlan(w http.ResponseWriter, change func(tx *sql.Tx, tournamentID int64) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	st, err := rotation.Load(tx)
	if err != nil {
		return err
	}
	if st.Tournament == nil {
		return errorf(http.StatusConflict, "Nessun torneo in corso: il programma delle piste c'è solo nei tornei")
	}
	if err := change(tx, st.Tournament.ID); err != nil {
		return err
	}
	if st, err = rotation.Load(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, st)
}
