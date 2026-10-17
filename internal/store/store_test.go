package store

import (
	"path/filepath"
	"testing"
)

// Le migrazioni girano senza chiavi esterne: dopo vanno riattivate su tutte le connessioni.
func TestForeignKeysAfterMigrations(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxIdleConns(4)

	for range 4 {
		var on int
		if err := db.QueryRow("PRAGMA foreign_keys").Scan(&on); err != nil {
			t.Fatal(err)
		}
		if on != 1 {
			t.Fatal("chiavi esterne disattivate dopo le migrazioni")
		}
	}

	// I nuovi tipi di sessione sono accettati, quelli sconosciuti no; eliminare un torneo elimina le sue gare.
	if _, err := db.Exec("INSERT INTO tournaments (id, name) VALUES (10, 'T')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO races (tournament_id, track, type) VALUES (10, 'Monza', 'practice')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO races (tournament_id, track, type) VALUES (10, 'Monza', 'altro')"); err == nil {
		t.Error("tipo di gara sconosciuto accettato")
	}
	if _, err := db.Exec("INSERT INTO races (tournament_id, track, type) VALUES (99, 'Monza', 'race')"); err == nil {
		t.Error("gara di un torneo inesistente accettata")
	}
	db.Exec("DELETE FROM tournaments WHERE id = 10")
	var races int
	db.QueryRow("SELECT COUNT(*) FROM races").Scan(&races)
	if races != 0 {
		t.Errorf("restano %d gare del torneo eliminato", races)
	}
}
