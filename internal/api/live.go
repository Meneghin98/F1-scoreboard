package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"f1-scoreboard/internal/live"
)

// getLive restituisce lo stato attuale della sessione in corso.
func (s *Server) getLive(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, err := w.Write(s.live.Snapshot())
	return err
}

// getLiveTrack restituisce la pianta del circuito della sessione in corso.
func (s *Server) getLiveTrack(w http.ResponseWriter, r *http.Request) error {
	m := s.live.TrackMap()
	if m == nil {
		return errorf(http.StatusNotFound, "Nessuna mappa disponibile")
	}
	w.Header().Set("Cache-Control", "no-store")
	return writeJSON(w, http.StatusOK, m)
}

// streamLive invia lo stato della sessione con Server-Sent Events: un messaggio
// a ogni cambiamento (al massimo 4 al secondo). Il browser lo riceve con
// EventSource, che si riconnette da solo se la connessione cade.
func (s *Server) streamLive(w http.ResponseWriter, r *http.Request) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.New("lo streaming non è supportato da questa connessione")
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no") // evita il buffering di eventuali proxy

	updates, unsubscribe := s.live.Subscribe()
	defer unsubscribe()

	// Un commento periodico tiene viva la connessione anche quando non arriva nulla.
	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return nil
		case <-s.live.Done():
			return nil
		case msg := <-updates:
			if _, err := fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
				return nil // client disconnesso
			}
		case <-keepAlive.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return nil
			}
		}
		flusher.Flush()
	}
}

func (s *Server) listTelemetrySessions(w http.ResponseWriter, r *http.Request) error {
	sessions, err := s.live.Sessions()
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, sessions)
}

// downloadTelemetrySession invia la registrazione di una sessione (.f1rec.gz).
func (s *Server) downloadTelemetrySession(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return errorf(http.StatusNotFound, "Sessione non trovata")
	}
	path, name, err := s.live.SessionFile(id)
	if errors.Is(err, live.ErrSessionNotFound) {
		return errorf(http.StatusNotFound, "Sessione non trovata")
	}
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	http.ServeFile(w, r, path)
	return nil
}

// sessionID legge il parametro {id} di una sessione registrata.
func sessionID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return 0, errorf(http.StatusNotFound, "Sessione non trovata")
	}
	return id, nil
}

// sessionError traduce gli errori del servizio di telemetria nelle risposte HTTP.
func sessionError(err error) error {
	var invalid *live.InvalidError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, live.ErrSessionNotFound):
		return errorf(http.StatusNotFound, "Sessione non trovata")
	case errors.Is(err, live.ErrSessionRecording):
		return errorf(http.StatusConflict, "La sessione è ancora in registrazione")
	case errors.As(err, &invalid):
		return errorf(http.StatusBadRequest, "%s", invalid.Error())
	}
	return err
}

func (s *Server) deleteTelemetrySession(w http.ResponseWriter, r *http.Request) error {
	id, err := sessionID(r)
	if err != nil {
		return err
	}
	if err := sessionError(s.live.DeleteSession(id)); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// setTelemetryPlayers cambia i piloti associati ai giocatori di una sessione.
// Body: { "players": [{ "carIndex": 3, "playerId": 1 }, { "carIndex": 5, "playerId": null }] }
func (s *Server) setTelemetryPlayers(w http.ResponseWriter, r *http.Request) error {
	id, err := sessionID(r)
	if err != nil {
		return err
	}
	var body struct {
		Players []struct {
			CarIndex int    `json:"carIndex"`
			PlayerID *int64 `json:"playerId"`
		} `json:"players"`
	}
	if err := readJSON(r, &body); err != nil {
		return err
	}
	players := map[int]*int64{}
	for _, p := range body.Players {
		players[p.CarIndex] = p.PlayerID
	}
	if err := sessionError(s.live.SetSessionPlayers(id, players)); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// setTelemetryRace collega una sessione a una gara del torneo.
// Body: { "raceId": 4 } per una gara esistente, { "raceId": null } per scollegarla,
// { "tournamentId": 2, "type": "race" } per creare la gara dalla registrazione.
func (s *Server) setTelemetryRace(w http.ResponseWriter, r *http.Request) error {
	id, err := sessionID(r)
	if err != nil {
		return err
	}
	var body struct {
		RaceID       *int64 `json:"raceId"`
		TournamentID *int64 `json:"tournamentId"`
		Type         string `json:"type"`
	}
	if err := readJSON(r, &body); err != nil {
		return err
	}
	if body.TournamentID != nil {
		_, err = s.live.LinkNewRace(id, *body.TournamentID, body.Type)
	} else {
		err = s.live.SetSessionRace(id, body.RaceID)
	}
	if err := sessionError(err); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
