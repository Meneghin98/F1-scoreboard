package live

import (
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Conservazione delle registrazioni. Una registrazione pesa circa 2 MB al
// minuto e su una scheda SD lo spazio finisce in fretta: dalla pagina Sistema
// si sceglie dopo quanti giorni eliminarne i file. Le sessioni restano, con i
// giocatori, i risultati delle gare e le statistiche, ma senza file non si
// possono più rivedere né scaricare.

// Retention sono le impostazioni di conservazione, salvate nella tabella settings.
type Retention struct {
	Days            int  `json:"days"`            // giorni dalla fine della sessione, 0 = per sempre
	KeepTournaments bool `json:"keepTournaments"` // non eliminare i file delle gare dei tornei
}

// Chiavi delle impostazioni di conservazione nella tabella settings.
const (
	settingRetentionDays   = "recordings_retention_days"
	settingKeepTournaments = "recordings_keep_tournaments"
)

// Retention restituisce le impostazioni di conservazione delle registrazioni.
func (s *Service) Retention() (Retention, error) {
	r := Retention{KeepTournaments: true}
	rows, err := s.db.Query("SELECT key, value FROM settings WHERE key IN (?, ?)", settingRetentionDays, settingKeepTournaments)
	if err != nil {
		return r, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return r, err
		}
		switch key {
		case settingRetentionDays:
			r.Days, _ = strconv.Atoi(value) // un valore non valido vale 0: si tiene tutto
		case settingKeepTournaments:
			r.KeepTournaments = value == "1"
		}
	}
	return r, rows.Err()
}

// SetRetention salva le impostazioni di conservazione ed elimina subito i file
// che non vanno più tenuti: restituisce quanti sono e quanto spazio occupavano.
func (s *Service) SetRetention(r Retention) (deleted int, freed int64, err error) {
	keep := "0"
	if r.KeepTournaments {
		keep = "1"
	}
	_, err = s.db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?), (?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`,
		settingRetentionDays, strconv.Itoa(r.Days), settingKeepTournaments, keep)
	if err != nil {
		return 0, 0, err
	}
	deleted, freed = s.deleteOldRecordings(time.Now())
	return deleted, freed, nil
}

// deleteOldRecordings elimina i file delle registrazioni finite da più giorni
// di quanto dicono le impostazioni (e, se richiesto, non collegate a una gara
// di un torneo). Le sessioni restano: senza file risultano "non disponibili".
func (s *Service) deleteOldRecordings(now time.Time) (deleted int, freed int64) {
	r, err := s.Retention()
	if err != nil {
		log.Printf("telemetria: impostazioni di conservazione: %v", err)
		return 0, 0
	}
	if r.Days <= 0 {
		return 0, 0
	}
	// Restano anche le sessioni che il server deve ancora leggere: da
	// analizzare, da salvare in un torneo o senza i dati delle statistiche.
	rows, err := s.db.Query(`
		SELECT s.id, s.file FROM telemetry_sessions s
		LEFT JOIN races r ON r.id = s.race_id
		LEFT JOIN tournaments t ON t.id = r.tournament_id
		WHERE s.ended_at IS NOT NULL AND s.ended_at < ?
		  AND s.analyzed = 1 AND s.pending_save = 0 AND NOT (s.has_classification = 1 AND s.stats_extracted = 0)
		  AND NOT (? AND COALESCE(t.free, 1) = 0)`,
		dbTime(now.AddDate(0, 0, -r.Days)), r.KeepTournaments)
	if err != nil {
		log.Printf("telemetria: %v", err)
		return 0, 0
	}
	type old struct {
		id   int64
		file string
	}
	var list []old
	for rows.Next() {
		var o old
		if rows.Scan(&o.id, &o.file) == nil {
			list = append(list, o)
		}
	}
	rows.Close()

	for _, o := range list {
		path := filepath.Join(s.opts.Dir, o.file)
		info, err := os.Stat(path)
		if err != nil {
			continue // file già eliminato
		}
		if err := os.Remove(path); err != nil {
			log.Printf("telemetria: eliminazione della registrazione %s: %v", o.file, err)
			continue
		}
		s.dropReplay(o.id, o.file) // senza registrazione il replay salvato non serve più
		deleted++
		freed += info.Size()
	}
	if deleted > 0 {
		log.Printf("telemetria: registrazioni più vecchie di %d giorni, file eliminati: %d (%.1f MB)", r.Days, deleted, float64(freed)/(1<<20))
		s.DataChanged()
	}
	return deleted, freed
}

// RecordingsUsage restituisce quante registrazioni ci sono nella cartella e
// quanto spazio occupa la cartella. Se non esiste ancora, zero.
func (s *Service) RecordingsUsage() (files int, size int64, err error) {
	entries, err := os.ReadDir(s.opts.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		size += info.Size()
		if strings.HasSuffix(e.Name(), ".f1rec.gz") {
			files++
		}
	}
	return files, size, nil
}

// Options restituisce la configurazione del servizio: cartella delle
// registrazioni e se le sessioni vengono registrate.
func (s *Service) Options() Options { return s.opts }
