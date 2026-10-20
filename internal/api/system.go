package api

import (
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"f1-scoreboard/internal/backup"
	"f1-scoreboard/internal/disk"
)

// API della pagina Sistema: backup del database, spazio su disco,
// conservazione delle registrazioni e versione del programma.

// downloadBackup invia una copia del database fatta in quel momento, con un
// nome che dice quando. Il file temporaneo si elimina dopo l'invio.
func (s *Server) downloadBackup(w http.ResponseWriter, r *http.Request) error {
	path, err := s.backups.Snapshot(r.Context())
	if err != nil {
		return err
	}
	defer os.Remove(path)
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close() // prima di os.Remove: su Windows un file aperto non si può eliminare
	info, err := f.Stat()
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", backup.Name(time.Now())))
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "", info.ModTime(), f)
	return nil
}

// listBackups restituisce i backup automatici, dal più recente.
func (s *Server) listBackups(w http.ResponseWriter, r *http.Request) error {
	list, err := s.backups.List()
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, list)
}

// downloadAutoBackup invia un backup automatico.
func (s *Server) downloadAutoBackup(w http.ResponseWriter, r *http.Request) error {
	name := r.PathValue("name")
	path, err := s.backups.Path(name)
	if errors.Is(err, backup.ErrNotFound) {
		return errorf(http.StatusNotFound, "Backup non trovato")
	}
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	http.ServeFile(w, r, path)
	return nil
}

// SystemInfo descrive il server alla pagina Sistema.
type SystemInfo struct {
	Version  Version `json:"version"`
	Database struct {
		Path      string `json:"path"`
		SizeBytes int64  `json:"sizeBytes"` // compreso il file WAL
	} `json:"database"`
	Recordings struct {
		Dir       string `json:"dir"`
		Enabled   bool   `json:"enabled"` // le sessioni vengono registrate (TELEMETRY_RECORD)
		Files     int    `json:"files"`
		SizeBytes int64  `json:"sizeBytes"` // tutta la cartella
	} `json:"recordings"`
	Backups struct {
		Dir       string `json:"dir"`
		Keep      int    `json:"keep"` // backup automatici tenuti, 0 se spenti
		Count     int    `json:"count"`
		SizeBytes int64  `json:"sizeBytes"`
	} `json:"backups"`
	Disk *disk.Usage `json:"disk"` // disco della cartella del database, nil se non si sa
}

// Version identifica l'eseguibile: il commit da cui è stato compilato.
type Version struct {
	Revision string `json:"revision"` // commit Git, vuoto se compilato senza Git (o con go run)
	Time     string `json:"time"`     // data del commit, RFC 3339
	Modified bool   `json:"modified"` // compilato con modifiche non committate
	Go       string `json:"go"`       // versione di Go
}

// buildVersion legge una volta sola le informazioni che go build scrive nell'eseguibile.
var buildVersion = sync.OnceValue(func() Version {
	var v Version
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return v
	}
	v.Go = info.GoVersion
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			v.Revision = s.Value
		case "vcs.time":
			v.Time = s.Value
		case "vcs.modified":
			v.Modified = s.Value == "true"
		}
	}
	return v
})

// getSystem restituisce versione e spazio occupato da database, registrazioni e backup.
func (s *Server) getSystem(w http.ResponseWriter, r *http.Request) error {
	info := SystemInfo{Version: buildVersion()}
	dbPath := s.backups.DBPath()
	info.Database.Path = absPath(dbPath)
	for _, path := range []string{dbPath, dbPath + "-wal"} {
		if st, err := os.Stat(path); err == nil {
			info.Database.SizeBytes += st.Size()
		}
	}

	opts := s.live.Options()
	info.Recordings.Dir, info.Recordings.Enabled = absPath(opts.Dir), opts.Record
	files, size, err := s.live.RecordingsUsage()
	if err != nil {
		return err
	}
	info.Recordings.Files, info.Recordings.SizeBytes = files, size

	info.Backups.Dir, info.Backups.Keep = absPath(s.backups.Dir()), s.backups.Keep()
	list, err := s.backups.List()
	if err != nil {
		return err
	}
	info.Backups.Count = len(list)
	for _, b := range list {
		info.Backups.SizeBytes += b.SizeBytes
	}

	if usage, err := disk.UsageOf(filepath.Dir(dbPath)); err == nil {
		info.Disk = &usage
	} else if !errors.Is(err, errors.ErrUnsupported) {
		log.Printf("spazio su disco: %v", err)
	}
	return writeJSON(w, http.StatusOK, info)
}

// absPath restituisce il percorso assoluto, più utile da mostrare (o quello dato, se non si può).
func absPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// maxRetentionDays è il massimo dei giorni di conservazione delle registrazioni (dieci anni).
const maxRetentionDays = 3650

// getSettings restituisce le impostazioni modificabili dalla pagina Sistema.
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) error {
	retention, err := s.live.Retention()
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"recordings": retention})
}

// saveSettings salva le impostazioni; i campi assenti restano come sono. Con la
// nuova conservazione i file delle registrazioni troppo vecchie si eliminano
// subito: la risposta dice quanti erano e quanto spazio occupavano.
// Body: { "recordings": { "days": 30, "keepTournaments": true } }
func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Recordings *struct {
			Days            *float64 `json:"days"` // float per poter rifiutare i decimali con un messaggio chiaro
			KeepTournaments *bool    `json:"keepTournaments"`
		} `json:"recordings"`
	}
	if err := readJSON(r, &body); err != nil {
		return err
	}
	retention, err := s.live.Retention()
	if err != nil {
		return err
	}
	if in := body.Recordings; in != nil {
		if d := in.Days; d != nil {
			if *d < 0 || *d > maxRetentionDays || *d != math.Trunc(*d) {
				return errorf(http.StatusBadRequest, "I giorni di conservazione delle registrazioni devono essere un numero intero da 0 a %d", maxRetentionDays)
			}
			retention.Days = int(*d)
		}
		if in.KeepTournaments != nil {
			retention.KeepTournaments = *in.KeepTournaments
		}
	}
	deleted, freed, err := s.live.SetRetention(retention)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"recordings": retention, "deletedFiles": deleted, "freedBytes": freed})
}
