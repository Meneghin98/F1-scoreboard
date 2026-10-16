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
