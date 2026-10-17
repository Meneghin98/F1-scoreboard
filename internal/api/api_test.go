package api_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"f1-scoreboard/internal/api"
	"f1-scoreboard/internal/live"
	"f1-scoreboard/internal/store"
)

// newTestServer avvia le API su un database temporaneo.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(api.New(db, fstest.MapFS{}, live.NewService(db, live.Options{Dir: t.TempDir()})))
	// I cleanup girano in ordine inverso: prima il server, poi il DB, poi la cartella temporanea.
	t.Cleanup(func() { db.Close() })
	t.Cleanup(srv.Close)
	return srv
}

// call esegue una richiesta e decodifica la risposta JSON (se presente) in out.
func call(t *testing.T, srv *httptest.Server, method, path string, body any, out any) int {
	t.Helper()
	var reqBody bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&reqBody).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, srv.URL+"/api"+path, &reqBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatalf("%s %s: risposta non JSON: %v", method, path, err)
		}
	}
	return res.StatusCode
}

func expectStatus(t *testing.T, got, want int, what string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: status %d, atteso %d", what, got, want)
	}
}

type result struct {
	PlayerID int  `json:"player_id"`
	Position *int `json:"position,omitempty"`
	DNF      bool `json:"dnf,omitempty"`
}

func pos(p int) *int { return &p }

// Stesso flusso del vecchio smoke test Node.js.
func TestMainFlow(t *testing.T) {
	srv := newTestServer(t)

	expectStatus(t, call(t, srv, "POST", "/tournaments", map[string]string{"name": "Test"}, nil), 201, "crea torneo")
	for _, name := range []string{"Simone", "Luca", "Marco"} {
		expectStatus(t, call(t, srv, "POST", "/players", map[string]string{"name": name, "team": "Ferrari"}, nil), 201, "crea pilota")
	}
	expectStatus(t, call(t, srv, "POST", "/players", map[string]string{"name": "simone"}, nil), 409, "nome duplicato")

	call(t, srv, "POST", "/tournaments/1/races", map[string]string{"track": "Monza", "type": "race"}, nil)
	call(t, srv, "POST", "/tournaments/1/races", map[string]string{"track": "Imola", "type": "sprint"}, nil)

	expectStatus(t, call(t, srv, "PUT", "/races/1/results", map[string]any{"results": []result{
		{PlayerID: 1, Position: pos(2)}, {PlayerID: 2, Position: pos(1)}, {PlayerID: 3, DNF: true},
	}}, nil), 204, "risultati GP")
	expectStatus(t, call(t, srv, "PUT", "/races/2/results", map[string]any{"results": []result{
		{PlayerID: 1, Position: pos(1)}, {PlayerID: 2, Position: pos(3)}, {PlayerID: 3, Position: pos(2)},
	}}, nil), 204, "risultati sprint")
	expectStatus(t, call(t, srv, "PUT", "/races/2/results", map[string]any{"results": []result{
		{PlayerID: 1, Position: pos(1)}, {PlayerID: 2, Position: pos(1)},
	}}, nil), 400, "posizione duplicata")
	expectStatus(t, call(t, srv, "DELETE", "/players/1", nil, nil), 409, "pilota con risultati")

	var data struct {
		Standings []struct {
			Rank   int    `json:"rank"`
			Name   string `json:"name"`
			Points int    `json:"points"`
		} `json:"standings"`
	}
	expectStatus(t, call(t, srv, "GET", "/tournaments/1/standings", nil, &data), 200, "classifica")

	var summary []string
	for _, s := range data.Standings {
		summary = append(summary, fmt.Sprintf("%d. %s %dpt", s.Rank, s.Name, s.Points))
	}
	// Simone: 18 + 8 = 26, Luca: 25 + 6 = 31, Marco: 0 + 7 = 7
	want := []string{"1. Luca 31pt", "2. Simone 26pt", "3. Marco 7pt"}
	if fmt.Sprint(summary) != fmt.Sprint(want) {
		t.Fatalf("classifica %v, attesa %v", summary, want)
	}
}

func TestRaceResultsAreSortedWithDNFLast(t *testing.T) {
	srv := newTestServer(t)
	call(t, srv, "POST", "/tournaments", map[string]string{"name": "Test"}, nil)
	for _, name := range []string{"A", "B", "C"} {
		call(t, srv, "POST", "/players", map[string]string{"name": name}, nil)
	}
	call(t, srv, "POST", "/tournaments/1/races", map[string]string{"track": "Monza"}, nil)
	call(t, srv, "PUT", "/races/1/results", map[string]any{"results": []result{
		{PlayerID: 1, DNF: true}, {PlayerID: 2, Position: pos(2)}, {PlayerID: 3, Position: pos(1)},
	}}, nil)

	var races []struct {
		Type    string `json:"type"`
		Results []struct {
			PlayerID int  `json:"player_id"`
			Points   int  `json:"points"`
			DNF      bool `json:"dnf"`
		} `json:"results"`
	}
	expectStatus(t, call(t, srv, "GET", "/tournaments/1/races", nil, &races), 200, "elenco gare")

	if races[0].Type != "race" {
		t.Errorf("tipo di default %q, atteso \"race\"", races[0].Type)
	}
	got := fmt.Sprint(races[0].Results)
	want := "[{3 25 false} {2 18 false} {1 0 true}]"
	if got != want {
		t.Fatalf("risultati %s, attesi %s", got, want)
	}
}

func TestErrors(t *testing.T) {
	srv := newTestServer(t)
	call(t, srv, "POST", "/tournaments", map[string]string{"name": "Test"}, nil)
	call(t, srv, "POST", "/players", map[string]string{"name": "A"}, nil)
	call(t, srv, "POST", "/tournaments/1/races", map[string]string{"track": "Monza"}, nil)

	tests := []struct {
		name, method, path string
		body               any
		want               int
	}{
		{"nome mancante", "POST", "/tournaments", map[string]string{"name": "  "}, 400},
		{"JSON non valido", "POST", "/tournaments", "non un oggetto", 400},
		{"torneo inesistente", "DELETE", "/tournaments/99", nil, 404},
		{"id non numerico", "GET", "/tournaments/abc/races", nil, 404},
		{"endpoint inesistente", "GET", "/nope", nil, 404},
		{"risultati mancanti", "PUT", "/races/1/results", map[string]any{}, 400},
		{"pilota inesistente", "PUT", "/races/1/results", map[string]any{"results": []result{{PlayerID: 9, Position: pos(1)}}}, 400},
		{"posizione decimale", "PUT", "/races/1/results", map[string]any{"results": []map[string]any{{"player_id": 1, "position": 1.5}}}, 400},
		{"posizione mancante", "PUT", "/races/1/results", map[string]any{"results": []result{{PlayerID: 1}}}, 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body struct {
				Error string `json:"error"`
			}
			expectStatus(t, call(t, srv, tt.method, tt.path, tt.body, &body), tt.want, tt.name)
			if body.Error == "" {
				t.Error("manca il messaggio di errore")
			}
		})
	}
}

func TestLiveEndpoints(t *testing.T) {
	srv := newTestServer(t)

	var snap struct {
		Status  string `json:"status"`
		Drivers []any  `json:"drivers"`
	}
	expectStatus(t, call(t, srv, "GET", "/live", nil, &snap), 200, "stato live")
	if snap.Status != "waiting" || snap.Drivers == nil {
		t.Errorf("stato senza telemetria: %+v", snap)
	}

	var sessions []any
	expectStatus(t, call(t, srv, "GET", "/telemetry/sessions", nil, &sessions), 200, "elenco sessioni")
	if sessions == nil || len(sessions) != 0 {
		t.Errorf("sessioni %v", sessions)
	}
	expectStatus(t, call(t, srv, "GET", "/telemetry/sessions/99/download", nil, nil), 404, "sessione inesistente")
	expectStatus(t, call(t, srv, "GET", "/telemetry/sessions/abc/download", nil, nil), 404, "id non numerico")
}

func TestLiveStream(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/live/stream", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type %q", ct)
	}
	// Il primo evento arriva subito, con lo stato attuale.
	line, err := bufio.NewReader(res.Body).ReadString('\n')
	if err != nil || !strings.HasPrefix(line, `data: {"status":"waiting"`) {
		t.Fatalf("primo evento %q, %v", line, err)
	}
}

func TestTelemetrySessionEndpoints(t *testing.T) {
	srv := newTestServer(t)
	var errBody map[string]string
	expectStatus(t, call(t, srv, "DELETE", "/telemetry/sessions/1", nil, &errBody), http.StatusNotFound, "eliminazione")
	expectStatus(t, call(t, srv, "PUT", "/telemetry/sessions/1/players", map[string]any{"players": []any{}}, &errBody),
		http.StatusNotFound, "piloti")
	expectStatus(t, call(t, srv, "PUT", "/telemetry/sessions/1/race", map[string]any{"raceId": nil}, &errBody),
		http.StatusNotFound, "gara")
	expectStatus(t, call(t, srv, "PUT", "/telemetry/sessions/abc/race", nil, &errBody), http.StatusNotFound, "id non valido")
}
