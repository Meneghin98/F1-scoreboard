// Package api espone le API REST del segnapunti e serve i file del frontend.
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"

	"f1-scoreboard/internal/live"
)

type Server struct {
	db   *sql.DB
	live *live.Service
}

// New restituisce l'handler HTTP completo: API sotto /api, file statici altrove.
func New(db *sql.DB, static fs.FS, liveService *live.Service) http.Handler {
	s := &Server{db: db, live: liveService}
	mux := http.NewServeMux()

	mux.Handle("GET /api/points", s.handle(s.getPoints))

	mux.Handle("GET /api/tournaments", s.handle(s.listTournaments))
	mux.Handle("POST /api/tournaments", s.handle(s.createTournament))
	mux.Handle("DELETE /api/tournaments/{id}", s.handle(s.deleteTournament))

	mux.Handle("GET /api/players", s.handle(s.listPlayers))
	mux.Handle("POST /api/players", s.handle(s.createPlayer))
	mux.Handle("DELETE /api/players/{id}", s.handle(s.deletePlayer))

	mux.Handle("GET /api/tournaments/{id}/races", s.handle(s.listRaces))
	mux.Handle("POST /api/tournaments/{id}/races", s.handle(s.createRace))
	mux.Handle("DELETE /api/races/{id}", s.handle(s.deleteRace))
	mux.Handle("PUT /api/races/{id}/results", s.handle(s.saveResults))

	mux.Handle("GET /api/tournaments/{id}/standings", s.handle(s.getStandings))

	mux.Handle("GET /api/live", s.handle(s.getLive))
	mux.Handle("GET /api/live/stream", s.handle(s.streamLive))
	mux.Handle("GET /api/live/track", s.handle(s.getLiveTrack))
	mux.Handle("GET /api/telemetry/sessions", s.handle(s.listTelemetrySessions))
	mux.Handle("GET /api/telemetry/sessions/{id}/download", s.handle(s.downloadTelemetrySession))
	mux.Handle("DELETE /api/telemetry/sessions/{id}", s.handle(s.deleteTelemetrySession))
	mux.Handle("PUT /api/telemetry/sessions/{id}/players", s.handle(s.setTelemetryPlayers))
	mux.Handle("PUT /api/telemetry/sessions/{id}/race", s.handle(s.setTelemetryRace))

	mux.Handle("/api/", s.handle(func(w http.ResponseWriter, r *http.Request) error {
		return errorf(http.StatusNotFound, "Endpoint non trovato")
	}))
	mux.Handle("/", staticHandler(static))

	return mux
}

// --- Gestione errori ------------------------------------------------------

// httpError è un errore da mostrare al client con il relativo status HTTP.
// Tutti gli altri errori diventano un 500 generico (e finiscono nel log).
type httpError struct {
	status  int
	message string
}

func (e *httpError) Error() string { return e.message }

func errorf(status int, format string, args ...any) error {
	return &httpError{status: status, message: fmt.Sprintf(format, args...)}
}

// handlerFunc è un handler che può restituire un errore, così non serve
// scrivere la risposta d'errore a mano in ogni punto.
type handlerFunc func(w http.ResponseWriter, r *http.Request) error

func (s *Server) handle(h handlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := h(w, r)
		if err == nil {
			return
		}
		var he *httpError
		if errors.As(err, &he) {
			writeJSON(w, he.status, map[string]string{"error": he.message})
			return
		}
		log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Errore interno del server"})
	})
}

// --- Utility --------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

// readJSON decodifica il body nella struct v. Un body vuoto equivale a {}.
func readJSON(r *http.Request, v any) error {
	err := json.NewDecoder(r.Body).Decode(v)
	if err != nil && !errors.Is(err, io.EOF) {
		return errorf(http.StatusBadRequest, "Body JSON non valido")
	}
	return nil
}

func requireText(value, field string) (string, error) {
	text := strings.TrimSpace(value)
	if text == "" {
		return "", errorf(http.StatusBadRequest, "Il campo \"%s\" è obbligatorio", field)
	}
	return text, nil
}

// optionalText restituisce nil per le stringhe vuote, così nel DB finisce NULL.
func optionalText(value string) *string {
	text := strings.TrimSpace(value)
	if text == "" {
		return nil
	}
	return &text
}

// pathID legge il parametro {id} dall'URL e verifica che la riga esista
// nella tabella indicata. what è il nome da usare nel messaggio di errore.
func (s *Server) pathID(r *http.Request, table, what string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err == nil {
		var found bool
		found, err = s.exists("SELECT 1 FROM "+table+" WHERE id = ?", id)
		if err != nil {
			return 0, err
		}
		if found {
			return id, nil
		}
	}
	return 0, errorf(http.StatusNotFound, "%s non trovato", what)
}

// exists dice se la query restituisce almeno una riga.
func (s *Server) exists(query string, args ...any) (bool, error) {
	var one int
	err := s.db.QueryRow(query, args...).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
