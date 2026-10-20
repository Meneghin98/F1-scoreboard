// Package backup fa le copie del database: su richiesta, da scaricare dalla
// pagina Sistema, e una al giorno in automatico nella cartella dei backup.
package backup

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"time"
)

const (
	// Il backup automatico si fa quando l'ultimo ha più di un giorno: si controlla ogni ora.
	every      = 24 * time.Hour
	checkEvery = time.Hour
	// File temporanei dei backup da scaricare, nella cartella del database.
	tempPattern = "backup-*.tmp"
	// Formato della data nel nome dei backup (ora locale del server).
	stampLayout = "2006-01-02_15-04"
)

// namePattern è il formato dei nomi dei backup: f1-scoreboard_AAAA-MM-GG_hh-mm.db.
var namePattern = regexp.MustCompile(`^f1-scoreboard_(\d{4}-\d{2}-\d{2}_\d{2}-\d{2})\.db$`)

// ErrNotFound indica un backup che non esiste (o un nome che non è di un backup).
var ErrNotFound = errors.New("backup non trovato")

// Name restituisce il nome del backup fatto all'istante t, uguale per quelli
// scaricati e per quelli automatici.
func Name(t time.Time) string {
	return "f1-scoreboard_" + t.Local().Format(stampLayout) + ".db"
}

// Manager fa i backup del database.
type Manager struct {
	db     *sql.DB
	dbPath string // file del database: i file temporanei vanno nella sua cartella
	dir    string // cartella dei backup automatici
	keep   int    // backup automatici da tenere, 0 per non farne
	done   chan struct{}
}

func New(db *sql.DB, dbPath, dir string, keep int) *Manager {
	return &Manager{db: db, dbPath: dbPath, dir: dir, keep: keep, done: make(chan struct{})}
}

// DBPath è il file del database, Dir la cartella dei backup automatici e Keep quanti se ne tengono.
func (m *Manager) DBPath() string { return m.dbPath }
func (m *Manager) Dir() string    { return m.dir }
func (m *Manager) Keep() int      { return m.keep }

// Done viene chiuso quando Run termina: dopo si può chiudere il database.
func (m *Manager) Done() <-chan struct{} { return m.done }

// Run fa il backup automatico all'avvio, se l'ultimo ha più di un giorno, e
// poi lo ricontrolla ogni ora, finché ctx non viene annullato.
func (m *Manager) Run(ctx context.Context) {
	defer close(m.done)
	m.removeTemp()
	if m.keep <= 0 {
		return
	}
	ticker := time.NewTicker(checkEvery)
	defer ticker.Stop()
	for {
		if err := m.daily(ctx, time.Now()); err != nil && ctx.Err() == nil {
			log.Printf("backup: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// daily fa il backup automatico se l'ultimo ha più di un giorno, poi elimina
// i più vecchi oltre keep.
func (m *Manager) daily(ctx context.Context, now time.Time) error {
	list, err := m.List()
	if err != nil {
		return err
	}
	if len(list) > 0 && now.Sub(list[0].created) < every {
		return nil
	}
	return m.Create(ctx, now)
}

// Create fa subito un backup nella cartella dei backup automatici, con l'ora
// now nel nome, poi elimina i più vecchi oltre keep (se i backup automatici
// sono attivi). Se nello stesso minuto c'è già un backup non fa nulla: è
// abbastanza recente. Si usa anche prima di installare un aggiornamento.
func (m *Manager) Create(ctx context.Context, now time.Time) error {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(m.dir, Name(now))
	// Il file si crea prima, vuoto, per non toccare un backup che ha già quel nome.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	f.Close()
	if err := m.vacuumInto(ctx, path); err != nil {
		os.Remove(path)
		return err
	}
	log.Printf("backup: creato %s", path)
	if m.keep <= 0 {
		return nil
	}
	return m.prune()
}

// prune elimina i backup automatici più vecchi, tenendo gli ultimi keep.
func (m *Manager) prune() error {
	list, err := m.List()
	if err != nil {
		return err
	}
	for _, b := range list[min(len(list), m.keep):] {
		if err := os.Remove(filepath.Join(m.dir, b.Name)); err != nil {
			return err
		}
	}
	return nil
}

// Snapshot copia il database in un file temporaneo nella sua cartella e ne
// restituisce il percorso: chi lo chiama lo elimina dopo averlo usato.
func (m *Manager) Snapshot(ctx context.Context) (string, error) {
	f, err := os.CreateTemp(filepath.Dir(m.dbPath), tempPattern)
	if err != nil {
		return "", err
	}
	path := f.Name()
	f.Close()
	if err := m.vacuumInto(ctx, path); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

// vacuumInto scrive in path (che non deve esistere o deve essere vuoto) una
// copia compatta del database. È una fotografia coerente anche mentre il
// server scrive, e comprende i dati ancora nel file WAL.
func (m *Manager) vacuumInto(ctx context.Context, path string) error {
	_, err := m.db.ExecContext(ctx, "VACUUM INTO ?", path)
	return err
}

// removeTemp elimina i file temporanei rimasti da un download interrotto
// (per esempio dallo spegnimento del server).
func (m *Manager) removeTemp() {
	dir := filepath.Dir(m.dbPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if ok, _ := filepath.Match(tempPattern, e.Name()); ok && e.Type().IsRegular() {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// Info è un backup automatico.
type Info struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"sizeBytes"`
	CreatedAt string `json:"createdAt"` // UTC, come datetime('now') di SQLite

	created time.Time
}

// List restituisce i backup automatici, dal più recente. L'ora è quella nel nome.
func (m *Manager) List() ([]Info, error) {
	entries, err := os.ReadDir(m.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []Info{}, nil
	}
	if err != nil {
		return nil, err
	}
	list := []Info{}
	for _, e := range entries {
		match := namePattern.FindStringSubmatch(e.Name())
		if match == nil || !e.Type().IsRegular() {
			continue
		}
		created, err := time.ParseInLocation(stampLayout, match[1], time.Local)
		if err != nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		list = append(list, Info{
			Name: e.Name(), SizeBytes: info.Size(), CreatedAt: created.UTC().Format("2006-01-02 15:04:05"), created: created,
		})
	}
	slices.SortFunc(list, func(a, b Info) int { return b.created.Compare(a.created) })
	return list, nil
}

// Path restituisce il percorso del backup automatico name. Accetta solo i nomi
// dei backup, così una richiesta non può leggere altri file.
func (m *Manager) Path(name string) (string, error) {
	if !namePattern.MatchString(name) {
		return "", ErrNotFound
	}
	path := filepath.Join(m.dir, name)
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return "", ErrNotFound
	}
	return path, nil
}
