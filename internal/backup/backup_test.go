package backup

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"f1-scoreboard/internal/store"
)

// newManager apre un database temporaneo con un pilota e un Manager che tiene keep backup.
func newManager(t *testing.T, keep int) *Manager {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scoreboard.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec("INSERT INTO players (name) VALUES ('Simone')"); err != nil {
		t.Fatal(err)
	}
	return New(db, dbPath, filepath.Join(dir, "backups"), keep)
}

// players conta i piloti nel database del file path.
func players(t *testing.T, path string) int {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM players").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSnapshot(t *testing.T) {
	m := newManager(t, 7)
	path, err := m.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != filepath.Dir(m.DBPath()) {
		t.Errorf("file temporaneo in %s, atteso accanto al database", path)
	}
	if n := players(t, path); n != 1 {
		t.Errorf("%d piloti nel backup, atteso 1", n)
	}
	// Un file temporaneo rimasto (download interrotto) si elimina al prossimo avvio.
	m.removeTemp()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file temporaneo non eliminato: %v", err)
	}
}

// Un backup al giorno, tenendo gli ultimi keep.
func TestDaily(t *testing.T) {
	m := newManager(t, 2)
	ctx := context.Background()
	start := time.Date(2026, 10, 20, 9, 30, 0, 0, time.Local)
	names := func() []string {
		list, err := m.List()
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, b := range list {
			out = append(out, b.Name)
		}
		return out
	}
	for _, at := range []time.Duration{0, time.Hour, 23 * time.Hour} {
		if err := m.daily(ctx, start.Add(at)); err != nil {
			t.Fatal(err)
		}
	}
	if got := names(); len(got) != 1 || got[0] != "f1-scoreboard_2026-10-20_09-30.db" {
		t.Fatalf("dopo meno di un giorno: %v", got)
	}
	m.daily(ctx, start.Add(24*time.Hour))
	m.daily(ctx, start.Add(49*time.Hour))
	got := names()
	if len(got) != 2 || got[0] != "f1-scoreboard_2026-10-22_10-30.db" || got[1] != "f1-scoreboard_2026-10-21_09-30.db" {
		t.Fatalf("dopo tre giorni: %v", got)
	}
	path, err := m.Path(got[0])
	if err != nil {
		t.Fatal(err)
	}
	if n := players(t, path); n != 1 {
		t.Errorf("%d piloti nel backup, atteso 1", n)
	}
}

func TestPathAcceptsOnlyBackupNames(t *testing.T) {
	m := newManager(t, 7)
	if err := m.daily(context.Background(), time.Date(2026, 1, 2, 3, 4, 0, 0, time.Local)); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Path("f1-scoreboard_2026-01-02_03-04.db"); err != nil {
		t.Errorf("backup esistente: %v", err)
	}
	// Nomi di altri file, o di backup che non esistono.
	for _, name := range []string{"../scoreboard.db", "scoreboard.db", "f1-scoreboard_x.db", "f1-scoreboard_2026-01-02_03-05.db"} {
		if _, err := m.Path(name); err != ErrNotFound {
			t.Errorf("%q: %v, atteso ErrNotFound", name, err)
		}
	}
}

// Con keep 0 i backup automatici sono spenti.
func TestRunWithoutAutomaticBackups(t *testing.T) {
	m := newManager(t, 0)
	m.Run(context.Background()) // torna subito
	if list, err := m.List(); err != nil || len(list) != 0 {
		t.Fatalf("backup %v, %v", list, err)
	}
}
