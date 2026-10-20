package api_test

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"f1-scoreboard/internal/api"
	"f1-scoreboard/internal/backup"
	"f1-scoreboard/internal/live"
	"f1-scoreboard/internal/store"
)

// newTestServer avvia le API su un database temporaneo.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv, _, _ := newTestServerDir(t)
	return srv
}

// newTestServerDB è newTestServer con accesso al database, per preparare dati
// che le API non scrivono (come quelli delle registrazioni).
func newTestServerDB(t *testing.T) (*httptest.Server, *sql.DB) {
	t.Helper()
	srv, db, _ := newTestServerDir(t)
	return srv, db
}

// newTestServerDir è newTestServerDB con la cartella dei dati: dentro ci sono
// il database (test.db), le registrazioni (sessions) e i backup (backups).
func newTestServerDir(t *testing.T) (*httptest.Server, *sql.DB, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	liveService := live.NewService(db, live.Options{Dir: filepath.Join(dir, "sessions")})
	backups := backup.New(db, dbPath, filepath.Join(dir, "backups"), 7)
	srv := httptest.NewServer(api.New(db, fstest.MapFS{}, liveService, backups))
	// I cleanup girano in ordine inverso: prima il server, poi il DB, poi la cartella temporanea.
	t.Cleanup(func() { db.Close() })
	t.Cleanup(srv.Close)
	return srv, db, dir
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
	// Il client Go chiede gzip da solo e decomprime: lo stream arriva compresso.
	if !res.Uncompressed {
		t.Error("stream non compresso con gzip")
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

func TestTournamentLifecycle(t *testing.T) {
	srv := newTestServer(t)
	type tournament struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Free   bool   `json:"free"`
		Active bool   `json:"active"`
	}
	list := func() []tournament {
		var out []tournament
		expectStatus(t, call(t, srv, "GET", "/tournaments", nil, &out), 200, "elenco tornei")
		return out
	}

	// "Nessun torneo" c'è sempre e non si può eliminare né iniziare.
	all := list()
	if len(all) != 1 || !all[0].Free {
		t.Fatalf("tornei iniziali %+v", all)
	}
	free := all[0].ID
	expectStatus(t, call(t, srv, "DELETE", fmt.Sprintf("/tournaments/%d", free), nil, nil), 409, "elimina torneo libero")
	expectStatus(t, call(t, srv, "POST", fmt.Sprintf("/tournaments/%d/start", free), nil, nil), 400, "inizia torneo libero")

	var a, b tournament
	expectStatus(t, call(t, srv, "POST", "/tournaments", map[string]any{"name": "A", "start": true}, &a), 201, "crea e inizia A")
	expectStatus(t, call(t, srv, "POST", "/tournaments", map[string]any{"name": "B"}, &b), 201, "crea B")
	if !a.Active || b.Active {
		t.Fatalf("A %+v, B %+v", a, b)
	}
	// Il torneo in corso è il primo dell'elenco, poi quello libero.
	if all := list(); all[0].ID != a.ID || all[1].ID != free {
		t.Fatalf("ordine %+v", all)
	}
	expectStatus(t, call(t, srv, "POST", fmt.Sprintf("/tournaments/%d/start", b.ID), nil, nil), 409, "due tornei in corso")
	expectStatus(t, call(t, srv, "POST", "/tournaments", map[string]any{"name": "C", "start": true}, nil), 409, "crea con un torneo in corso")

	expectStatus(t, call(t, srv, "POST", fmt.Sprintf("/tournaments/%d/end", a.ID), nil, &a), 200, "termina A")
	expectStatus(t, call(t, srv, "POST", fmt.Sprintf("/tournaments/%d/end", a.ID), nil, nil), 409, "termina due volte")
	expectStatus(t, call(t, srv, "POST", fmt.Sprintf("/tournaments/%d/start", b.ID), nil, &b), 200, "inizia B")
	if a.Active || !b.Active {
		t.Fatalf("A %+v, B %+v", a, b)
	}
}

func TestLiveLineupWithoutRace(t *testing.T) {
	srv := newTestServer(t)
	expectStatus(t, call(t, srv, "PUT", "/live/lineup", map[string]any{"confirm": true}, nil), 409, "nessuna gara")
}

// Piloti, tornei e gare si possono modificare; i circuiti noti hanno il loro identificativo.
func TestEditEndpoints(t *testing.T) {
	srv := newTestServer(t)
	type player struct {
		ID   int     `json:"id"`
		Name string  `json:"name"`
		Team *string `json:"team"`
	}
	var simone, luca player
	call(t, srv, "POST", "/players", map[string]string{"name": "Simone"}, &simone)
	call(t, srv, "POST", "/players", map[string]string{"name": "Luca"}, &luca)
	expectStatus(t, call(t, srv, "PUT", fmt.Sprintf("/players/%d", luca.ID), map[string]string{"name": "SIMONE"}, nil), 409, "nome già usato")
	expectStatus(t, call(t, srv, "PUT", fmt.Sprintf("/players/%d", simone.ID), map[string]string{"name": "simone", "team": "Ferrari"}, &simone),
		200, "stesso nome con altre maiuscole")
	if simone.Name != "simone" || simone.Team == nil || *simone.Team != "Ferrari" {
		t.Fatalf("pilota modificato %+v", simone)
	}
	expectStatus(t, call(t, srv, "PUT", "/players/99", map[string]string{"name": "X"}, nil), 404, "pilota inesistente")

	type tournament struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Free  bool   `json:"free"`
		Races int    `json:"races"`
	}
	var cup tournament
	call(t, srv, "POST", "/tournaments", map[string]any{"name": "Test"}, &cup)
	expectStatus(t, call(t, srv, "PUT", fmt.Sprintf("/tournaments/%d", cup.ID), map[string]string{"name": " Campionato "}, &cup), 200, "rinomina")
	if cup.Name != "Campionato" {
		t.Fatalf("torneo rinominato %+v", cup)
	}
	expectStatus(t, call(t, srv, "PUT", fmt.Sprintf("/tournaments/%d", cup.ID), map[string]string{"name": " "}, nil), 400, "nome vuoto")
	var all []tournament
	call(t, srv, "GET", "/tournaments", nil, &all)
	for _, t2 := range all {
		if t2.Free {
			expectStatus(t, call(t, srv, "PUT", fmt.Sprintf("/tournaments/%d", t2.ID), map[string]string{"name": "X"}, nil), 400, "rinomina Nessun torneo")
		}
	}

	type race struct {
		ID      int    `json:"id"`
		Track   string `json:"track"`
		TrackID *int   `json:"track_id"`
		Type    string `json:"type"`
	}
	races := fmt.Sprintf("/tournaments/%d/races", cup.ID)
	expectStatus(t, call(t, srv, "POST", races, map[string]string{"track": "Monza", "type": "altro"}, nil), 400, "tipo sconosciuto")
	var monza race
	expectStatus(t, call(t, srv, "POST", races, map[string]string{"track": "italia – monza"}, &monza), 201, "gara")
	if monza.TrackID == nil || *monza.TrackID != 11 || monza.Type != "race" {
		t.Fatalf("gara creata %+v", monza)
	}
	expectStatus(t, call(t, srv, "PUT", fmt.Sprintf("/races/%d", monza.ID),
		map[string]string{"track": "Miami", "type": "sprint", "date": "2026-05-03"}, &monza), 200, "modifica gara")
	if monza.Track != "Miami" || monza.TrackID == nil || *monza.TrackID != 30 || monza.Type != "sprint" {
		t.Fatalf("gara modificata %+v", monza)
	}
	call(t, srv, "GET", "/tournaments", nil, &all)
	if all[1].ID != cup.ID || all[1].Races != 1 {
		t.Fatalf("conteggio delle gare %+v", all)
	}
	var deleted map[string]int
	expectStatus(t, call(t, srv, "DELETE", fmt.Sprintf("/races/%d", monza.ID), nil, &deleted), 200, "elimina gara")
	if moved, ok := deleted["movedRecordings"]; !ok || moved != 0 {
		t.Fatalf("risposta all'eliminazione %v", deleted)
	}
	expectStatus(t, call(t, srv, "POST", fmt.Sprintf("/races/%d/resync", monza.ID), nil, nil), 404, "ripristino di una gara eliminata")

	var tracks []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	expectStatus(t, call(t, srv, "GET", "/tracks", nil, &tracks), 200, "circuiti")
	if len(tracks) < 24 || tracks[0].Name != "Australia – Melbourne" {
		t.Fatalf("circuiti %+v", tracks)
	}
}

// Dopo ogni modifica cambia la versione dei dati dello stato live: le pagine aperte si ricaricano.
func TestDataVersionChanges(t *testing.T) {
	srv := newTestServer(t)
	version := func() int64 {
		var snap struct {
			DataVersion int64 `json:"dataVersion"`
		}
		call(t, srv, "GET", "/live", nil, &snap)
		return snap.DataVersion
	}
	before := version()
	call(t, srv, "GET", "/players", nil, nil)
	if version() != before {
		t.Fatal("una lettura ha cambiato la versione")
	}
	call(t, srv, "POST", "/players", map[string]string{"name": "Simone"}, nil)
	if version() == before {
		t.Fatal("la versione non cambia dopo una modifica")
	}
}

// Ogni pilota ha un colore: il primo libero della tavolozza, o quello scelto.
func TestPlayerColors(t *testing.T) {
	srv := newTestServer(t)
	type player struct {
		ID    int    `json:"id"`
		Color string `json:"color"`
	}
	var a, b, c player
	call(t, srv, "POST", "/players", map[string]string{"name": "A"}, &a)
	call(t, srv, "POST", "/players", map[string]string{"name": "B", "color": "#ABCDEF"}, &b)
	call(t, srv, "POST", "/players", map[string]string{"name": "C"}, &c)
	if a.Color != "#e8443a" || b.Color != "#abcdef" || c.Color != "#3987e5" {
		t.Fatalf("colori %q, %q, %q", a.Color, b.Color, c.Color)
	}
	expectStatus(t, call(t, srv, "POST", "/players", map[string]string{"name": "D", "color": "rosso"}, nil), 400, "colore non valido")
	call(t, srv, "PUT", fmt.Sprintf("/players/%d", a.ID), map[string]string{"name": "A"}, &a)
	if a.Color != "#e8443a" {
		t.Fatalf("senza colore nel body il colore cambia: %q", a.Color)
	}
	call(t, srv, "PUT", fmt.Sprintf("/players/%d", a.ID), map[string]string{"name": "A", "color": "#00ff00"}, &a)
	if a.Color != "#00ff00" {
		t.Fatalf("colore non cambiato: %q", a.Color)
	}
}

// L'elenco dei tornei dice chi è in testa a ciascuno.
func TestTournamentLeader(t *testing.T) {
	srv := newTestServer(t)
	var cup struct {
		ID int `json:"id"`
	}
	call(t, srv, "POST", "/tournaments", map[string]string{"name": "Coppa"}, &cup)
	call(t, srv, "POST", "/players", map[string]string{"name": "Simone"}, nil)
	call(t, srv, "POST", "/players", map[string]string{"name": "Luca"}, nil)
	var race struct {
		ID int `json:"id"`
	}
	call(t, srv, "POST", fmt.Sprintf("/tournaments/%d/races", cup.ID), map[string]string{"track": "Monza"}, &race)
	call(t, srv, "PUT", fmt.Sprintf("/races/%d/results", race.ID), map[string]any{"results": []result{
		{PlayerID: 1, Position: pos(2)}, {PlayerID: 2, Position: pos(1)},
	}}, nil)
	var list []struct {
		ID     int `json:"id"`
		Leader *struct {
			Name   string `json:"name"`
			Color  string `json:"color"`
			Points int    `json:"points"`
		} `json:"leader"`
	}
	call(t, srv, "GET", "/tournaments", nil, &list)
	for _, t2 := range list {
		if t2.ID == cup.ID && (t2.Leader == nil || t2.Leader.Name != "Luca" || t2.Leader.Points != 25 || t2.Leader.Color == "") {
			t.Fatalf("capoclassifica %+v", t2.Leader)
		}
		if t2.ID != cup.ID && t2.Leader != nil {
			t.Errorf("torneo %d senza gare con capoclassifica %+v", t2.ID, t2.Leader)
		}
	}
}

// Le regole di punteggio si scelgono creando il torneo e si cambiano dopo: la
// classifica e i punti delle gare si ricalcolano con le nuove regole.
func TestTournamentRules(t *testing.T) {
	srv := newTestServer(t)
	type tournament struct {
		ID               int    `json:"id"`
		Name             string `json:"name"`
		Free             bool   `json:"free"`
		PointsMode       string `json:"points_mode"`
		PolePoints       int    `json:"pole_points"`
		FastestLapPoints int    `json:"fastest_lap_points"`
		PointsPreset     string `json:"points_preset"`
		RacePoints       []int  `json:"race_points"`
		SprintPoints     []int  `json:"sprint_points"`
		Leader           *struct {
			Name   string `json:"name"`
			Points int    `json:"points"`
		} `json:"leader"`
	}
	var std, cup tournament
	expectStatus(t, call(t, srv, "POST", "/tournaments", map[string]any{"name": "Standard"}, &std), 201, "torneo senza regole")
	if std.PointsMode != "absolute" || std.PolePoints != 0 || std.FastestLapPoints != 0 || std.PointsPreset != "f1" {
		t.Fatalf("regole di default %+v", std)
	}
	for name, body := range map[string]map[string]any{
		"modalità sconosciuta": {"name": "X", "points_mode": "ia"},
		"punti negativi":       {"name": "X", "pole_points": -1},
		"punti decimali":       {"name": "X", "fastest_lap_points": 1.5},
		"troppi punti":         {"name": "X", "pole_points": 26},
		"tabella vuota":        {"name": "X", "race_points": []int{}},
		"tabella senza punti":  {"name": "X", "race_points": []int{0, 0}},
		"tabella crescente":    {"name": "X", "race_points": []int{10, 12}},
		"tabella decimale":     {"name": "X", "sprint_points": []float64{3.5}},
		"tabella sconosciuta":  {"name": "X", "points_preset": "a caso"},
	} {
		expectStatus(t, call(t, srv, "POST", "/tournaments", body, nil), 400, name)
	}
	expectStatus(t, call(t, srv, "POST", "/tournaments",
		map[string]any{"name": "Coppa", "points_mode": "players", "pole_points": 3, "fastest_lap_points": 1}, &cup), 201, "torneo con regole")
	if cup.PointsMode != "players" || cup.PolePoints != 3 || cup.FastestLapPoints != 1 {
		t.Fatalf("regole %+v", cup)
	}

	// Simone e Luca corrono con la IA: in qualifica partono 4° e 7°, in gara arrivano 12° e 9°.
	for _, name := range []string{"Simone", "Luca"} {
		call(t, srv, "POST", "/players", map[string]string{"name": name}, nil)
	}
	var quali, race struct {
		ID int `json:"id"`
	}
	call(t, srv, "POST", fmt.Sprintf("/tournaments/%d/races", cup.ID), map[string]string{"track": "Monza", "type": "qualifying"}, &quali)
	call(t, srv, "POST", fmt.Sprintf("/tournaments/%d/races", cup.ID), map[string]string{"track": "Monza"}, &race)
	call(t, srv, "PUT", fmt.Sprintf("/races/%d/results", quali.ID), map[string]any{"results": []result{
		{PlayerID: 1, Position: pos(4)}, {PlayerID: 2, Position: pos(7)},
	}}, nil)
	call(t, srv, "PUT", fmt.Sprintf("/races/%d/results", race.ID), map[string]any{"results": []map[string]any{
		{"player_id": 1, "position": 12, "fastest_lap": true}, {"player_id": 2, "position": 9},
	}}, nil)

	type standings struct {
		Races []struct {
			ID   int    `json:"id"`
			Type string `json:"type"`
		} `json:"races"`
		Standings []struct {
			Name    string `json:"name"`
			Points  int    `json:"points"`
			Wins    int    `json:"wins"`
			Poles   int    `json:"poles"`
			PerRace map[string]struct {
				Points   int  `json:"points"`
				Position *int `json:"position"`
				Bonus    int  `json:"bonus"`
			} `json:"perRace"`
		} `json:"standings"`
	}
	var st standings
	call(t, srv, "GET", fmt.Sprintf("/tournaments/%d/standings", cup.ID), nil, &st)
	// Tra i piloti: Luca vince la gara (25), Simone fa la pole (+3) e il giro veloce (18 + 1).
	if len(st.Races) != 2 || st.Races[0].Type != "qualifying" {
		t.Fatalf("con i punti per la pole la qualifica entra in classifica: %+v", st.Races)
	}
	luca, simone := st.Standings[0], st.Standings[1]
	if luca.Name != "Luca" || luca.Points != 25 || luca.Wins != 1 || simone.Points != 3+18+1 || simone.Poles != 1 {
		t.Fatalf("classifica %+v", st.Standings)
	}
	if r := simone.PerRace[fmt.Sprint(race.ID)]; r.Points != 19 || *r.Position != 2 || r.Bonus != 1 {
		t.Errorf("gara di Simone %+v", r)
	}
	// Anche i punti di carriera vengono dalle classifiche dei tornei, con le loro regole.
	var stats struct {
		Career []struct {
			Name   string `json:"name"`
			Points int    `json:"points"`
		} `json:"career"`
	}
	call(t, srv, "GET", "/stats", nil, &stats)
	if len(stats.Career) != 2 || stats.Career[0].Name != "Luca" || stats.Career[0].Points != 25 || stats.Career[1].Points != 22 {
		t.Errorf("carriera %+v", stats.Career)
	}

	var races []struct {
		ID      int `json:"id"`
		Results []struct {
			PlayerID         int  `json:"player_id"`
			Position         *int `json:"position"`
			Points           int  `json:"points"`
			ScoredPosition   *int `json:"scored_position"`
			ScoredFastestLap bool `json:"scored_fastest_lap"`
			Bonus            int  `json:"bonus"`
		} `json:"results"`
	}
	call(t, srv, "GET", fmt.Sprintf("/tournaments/%d/races", cup.ID), nil, &races)
	if r := races[1].Results[1]; r.PlayerID != 1 || *r.Position != 12 || *r.ScoredPosition != 2 || r.Points != 19 || !r.ScoredFastestLap || r.Bonus != 1 {
		t.Errorf("risultato di Simone %+v", r)
	}

	// Si cambia solo la modalità: i bonus restano. Con le posizioni assolute nessuno fa punti in gara.
	expectStatus(t, call(t, srv, "PUT", fmt.Sprintf("/tournaments/%d", cup.ID), map[string]any{"points_mode": "absolute"}, &cup), 200, "cambia modalità")
	if cup.Name != "Coppa" || cup.PointsMode != "absolute" || cup.PolePoints != 3 {
		t.Fatalf("torneo modificato %+v", cup)
	}
	call(t, srv, "GET", fmt.Sprintf("/tournaments/%d/standings", cup.ID), nil, &st)
	if s := st.Standings[0]; s.Name != "Luca" || s.Points != 2 {
		t.Fatalf("classifica assoluta %+v", st.Standings)
	}
	expectStatus(t, call(t, srv, "PUT", fmt.Sprintf("/tournaments/%d", cup.ID), map[string]any{"pole_points": 30}, nil), 400, "troppi punti")

	var list []tournament
	call(t, srv, "GET", "/tournaments", nil, &list)
	for _, t2 := range list {
		if t2.ID == cup.ID && (t2.Leader == nil || t2.Leader.Name != "Luca" || t2.Leader.Points != 2) {
			t.Errorf("capoclassifica %+v", t2.Leader)
		}
		if t2.Free {
			expectStatus(t, call(t, srv, "PUT", fmt.Sprintf("/tournaments/%d", t2.ID), map[string]any{"pole_points": 1}, nil), 400, "regole di Nessun torneo")
		}
	}

	// Tra i piloti con i punti scelti: Luca primo prende 10, Simone secondo 5, più pole e giro veloce.
	// Gli zeri in fondo si tolgono e la tabella F1 resta il default.
	expectStatus(t, call(t, srv, "PUT", fmt.Sprintf("/tournaments/%d", cup.ID), map[string]any{
		"points_mode": "players", "points_preset": "custom", "race_points": []int{10, 5, 0}, "sprint_points": []int{8, 7, 6, 5, 4, 3, 2, 1},
	}, &cup), 200, "punti per posizione")
	if !slices.Equal(cup.RacePoints, []int{10, 5}) || cup.SprintPoints != nil {
		t.Fatalf("tabelle dei punti %v %v", cup.RacePoints, cup.SprintPoints)
	}
	call(t, srv, "GET", fmt.Sprintf("/tournaments/%d/standings", cup.ID), nil, &st)
	if luca, simone := st.Standings[0], st.Standings[1]; luca.Points != 10 || simone.Points != 3+5+1 {
		t.Fatalf("classifica con i punti scelti %+v", st.Standings)
	}
	// Cambiando altro, le tabelle restano.
	call(t, srv, "PUT", fmt.Sprintf("/tournaments/%d", cup.ID), map[string]any{"pole_points": 0}, &cup)
	if !slices.Equal(cup.RacePoints, []int{10, 5}) || cup.PointsPreset != "custom" {
		t.Errorf("tabella dopo la modifica %s %v", cup.PointsPreset, cup.RacePoints)
	}

	// Con Rank Down in una gara di 2 piloti il primo prende 2 punti e il secondo 1 (più il giro veloce).
	expectStatus(t, call(t, srv, "PUT", fmt.Sprintf("/tournaments/%d", cup.ID), map[string]any{"points_preset": "rank_down"}, &cup), 200, "Rank Down")
	call(t, srv, "GET", fmt.Sprintf("/tournaments/%d/standings", cup.ID), nil, &st)
	if luca, simone := st.Standings[0], st.Standings[1]; luca.Points != 2 || simone.Points != 1+1 {
		t.Fatalf("classifica con Rank Down %+v", st.Standings)
	}
}
