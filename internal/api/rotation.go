package api

import (
	"net/http"
	"slices"

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
