package api_test

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"f1-scoreboard/internal/api"
)

// get esegue una GET e restituisce la risposta con il corpo già letto.
func get(t *testing.T, url string) (*http.Response, []byte) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res, body
}

// Il backup da scaricare è una copia del database, con la data nel nome; il
// file temporaneo non resta nella cartella dei dati.
func TestDownloadBackup(t *testing.T) {
	srv, _, dir := newTestServerDir(t)
	call(t, srv, "POST", "/players", map[string]string{"name": "Simone"}, nil)

	res, body := get(t, srv.URL+"/api/backup")
	if res.StatusCode != 200 || !bytes.HasPrefix(body, []byte("SQLite format 3\x00")) {
		t.Fatalf("status %d, %d byte", res.StatusCode, len(body))
	}
	disposition := regexp.MustCompile(`^attachment; filename="f1-scoreboard_\d{4}-\d{2}-\d{2}_\d{2}-\d{2}\.db"$`)
	if cd := res.Header.Get("Content-Disposition"); !disposition.MatchString(cd) {
		t.Errorf("Content-Disposition %q", cd)
	}
	if tmp, _ := filepath.Glob(filepath.Join(dir, "backup-*.tmp")); len(tmp) > 0 {
		t.Errorf("file temporanei rimasti: %v", tmp)
	}
}

// I backup automatici si elencano e si scaricano, ma solo con i nomi dei backup.
func TestAutomaticBackups(t *testing.T) {
	srv, _, dir := newTestServerDir(t)
	var list []struct {
		Name      string `json:"name"`
		SizeBytes int    `json:"sizeBytes"`
		CreatedAt string `json:"createdAt"`
	}
	expectStatus(t, call(t, srv, "GET", "/backups", nil, &list), 200, "elenco vuoto")
	if list == nil || len(list) != 0 {
		t.Fatalf("backup %v", list)
	}

	backups := filepath.Join(dir, "backups")
	if err := os.MkdirAll(backups, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"f1-scoreboard_2026-10-19_09-30.db", "f1-scoreboard_2026-10-20_09-30.db", "appunti.txt"} {
		if err := os.WriteFile(filepath.Join(backups, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	call(t, srv, "GET", "/backups", nil, &list)
	if len(list) != 2 || list[0].Name != "f1-scoreboard_2026-10-20_09-30.db" || list[0].SizeBytes != 33 || list[0].CreatedAt == "" {
		t.Fatalf("backup %+v", list)
	}

	res, body := get(t, srv.URL+"/api/backups/"+list[1].Name)
	if res.StatusCode != 200 || string(body) != list[1].Name {
		t.Fatalf("download: status %d, %q", res.StatusCode, body)
	}
	for _, name := range []string{"appunti.txt", "test.db", "..%2Ftest.db", "f1-scoreboard_2026-10-21_09-30.db"} {
		if res, _ := get(t, srv.URL+"/api/backups/"+name); res.StatusCode != 404 {
			t.Errorf("%s: status %d", name, res.StatusCode)
		}
	}
}

func TestSystemInfo(t *testing.T) {
	srv, _, dir := newTestServerDir(t)
	api.SetInstallDir(t, dir)
	var info struct {
		Database struct {
			Path      string `json:"path"`
			SizeBytes int    `json:"sizeBytes"`
		} `json:"database"`
		Recordings struct {
			Dir     string `json:"dir"`
			Enabled bool   `json:"enabled"`
			Files   int    `json:"files"`
		} `json:"recordings"`
		Backups struct {
			Keep  int `json:"keep"`
			Count int `json:"count"`
		} `json:"backups"`
		Disk *struct {
			FreeBytes  int64 `json:"freeBytes"`
			TotalBytes int64 `json:"totalBytes"`
		} `json:"disk"`
	}
	expectStatus(t, call(t, srv, "GET", "/system", nil, &info), 200, "sistema")
	if info.Database.Path != "test.db" || info.Database.SizeBytes == 0 {
		t.Errorf("database %+v", info.Database)
	}
	if info.Recordings.Dir != "sessions" || info.Recordings.Enabled || info.Recordings.Files != 0 {
		t.Errorf("registrazioni %+v", info.Recordings)
	}
	if info.Backups.Keep != 7 || info.Backups.Count != 0 {
		t.Errorf("backup %+v", info.Backups)
	}
	if info.Disk == nil || info.Disk.FreeBytes <= 0 || info.Disk.TotalBytes < info.Disk.FreeBytes {
		t.Errorf("disco %+v", info.Disk)
	}
}

func TestSettings(t *testing.T) {
	srv := newTestServer(t)
	type settings struct {
		Recordings struct {
			Days            int  `json:"days"`
			KeepTournaments bool `json:"keepTournaments"`
		} `json:"recordings"`
		DeletedFiles *int `json:"deletedFiles"`
	}
	var s settings
	expectStatus(t, call(t, srv, "GET", "/settings", nil, &s), 200, "impostazioni")
	if s.Recordings.Days != 0 || !s.Recordings.KeepTournaments {
		t.Fatalf("impostazioni di default %+v", s)
	}
	for _, days := range []any{-1, 1.5, 4000, "30"} {
		body := map[string]any{"recordings": map[string]any{"days": days}}
		expectStatus(t, call(t, srv, "PUT", "/settings", body, nil), 400, "giorni non validi")
	}
	expectStatus(t, call(t, srv, "PUT", "/settings", map[string]any{"recordings": map[string]any{"days": 30}}, &s), 200, "salva")
	if s.Recordings.Days != 30 || !s.Recordings.KeepTournaments || s.DeletedFiles == nil || *s.DeletedFiles != 0 {
		t.Fatalf("impostazioni salvate %+v", s)
	}
	call(t, srv, "PUT", "/settings", map[string]any{"recordings": map[string]any{"keepTournaments": false}}, nil)
	call(t, srv, "GET", "/settings", nil, &s)
	if s.Recordings.Days != 30 || s.Recordings.KeepTournaments {
		t.Fatalf("impostazioni rilette %+v", s)
	}
}
