// Package store apre il database SQLite e lo porta all'ultima versione dello schema.
package store

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // driver SQLite in Go puro (niente CGO)
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open apre (o crea) il database in path e applica le migrazioni mancanti.
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	// I _pragma vengono applicati a ogni nuova connessione del pool:
	// foreign_keys in SQLite vale per connessione, non per database.
	// _txlock=immediate prende subito il lock in scrittura a inizio transazione,
	// così busy_timeout attende invece di fallire con "database is locked".
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrazione del database: %w", err)
	}
	return db, nil
}

// migrate esegue in ordine i file migrations/NNN_*.sql non ancora applicati.
// La versione corrente è salvata in PRAGMA user_version: il file N-esimo
// (in ordine alfabetico) porta il database alla versione N, quindi i file
// vanno numerati in sequenza e mai modificati dopo il rilascio.
func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}

	files, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}

	for i, name := range files {
		target := i + 1
		if target <= version {
			continue
		}
		script, err := migrations.ReadFile(name)
		if err != nil {
			return err
		}

		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(script)); err != nil {
			tx.Rollback()
			return fmt.Errorf("%s: %w", name, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", target)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
