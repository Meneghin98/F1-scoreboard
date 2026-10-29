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

	"f1-scoreboard/internal/telemetry"
)

// Conservazione delle registrazioni. Una registrazione pesa circa 2 MB al
// minuto e su una scheda SD lo spazio finisce in fretta: dalla pagina Sistema
// si sceglie dopo quanti giorni eliminarne i file. Le sessioni restano, con i
// giocatori, i risultati delle gare, le statistiche e i record, ma senza file
// non si possono più rivedere né scaricare. Le registrazioni dei record si
// tengono, salvo che lo si chieda: così il giro del record si può sempre rivedere.

// Retention sono le impostazioni di conservazione, salvate nella tabella settings.
type Retention struct {
	Days        int  `json:"days"`        // giorni dalla fine della sessione, 0 = per sempre
	KeepRecords bool `json:"keepRecords"` // non eliminare i file delle sessioni che detengono un record
}

// Chiavi delle impostazioni di conservazione nella tabella settings.
const (
	settingRetentionDays = "recordings_retention_days"
	settingKeepRecords   = "recordings_keep_records"
)

// Retention restituisce le impostazioni di conservazione delle registrazioni.
func (s *Service) Retention() (Retention, error) {
	r := Retention{KeepRecords: true}
	rows, err := s.db.Query("SELECT key, value FROM settings WHERE key IN (?, ?)", settingRetentionDays, settingKeepRecords)
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
		case settingKeepRecords:
			r.KeepRecords = value == "1"
		}
	}
	return r, rows.Err()
}

// SetRetention salva le impostazioni di conservazione ed elimina subito i file
// che non vanno più tenuti: restituisce quanti sono e quanto spazio occupavano.
func (s *Service) SetRetention(r Retention) (deleted int, freed int64, err error) {
	keep := "0"
	if r.KeepRecords {
		keep = "1"
	}
	_, err = s.db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?), (?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`,
		settingRetentionDays, strconv.Itoa(r.Days), settingKeepRecords, keep)
	if err != nil {
		return 0, 0, err
	}
	deleted, freed = s.deleteOldRecordings(time.Now())
	return deleted, freed, nil
}

// deleteOldRecordings elimina i file delle registrazioni finite da più giorni
// di quanto dicono le impostazioni (e, se richiesto, che non detengono un
// record). Le sessioni restano: senza file risultano "non disponibili".
func (s *Service) deleteOldRecordings(now time.Time) (deleted int, freed int64) {
	r, err := s.Retention()
	if err != nil {
		log.Printf("telemetria: impostazioni di conservazione: %v", err)
		return 0, 0
	}
	if r.Days <= 0 {
		return 0, 0
	}
	records := map[int64]bool{}
	if r.KeepRecords {
		if records, err = s.recordSessions(); err != nil {
			log.Printf("telemetria: sessioni dei record: %v", err)
			return 0, 0 // senza sapere quali sono, non si elimina niente
		}
	}
	// Restano anche le sessioni che il server deve ancora leggere: da
	// analizzare, da salvare in un torneo o senza i dati delle statistiche.
	rows, err := s.db.Query(`
		SELECT id, file FROM telemetry_sessions
		WHERE ended_at IS NOT NULL AND ended_at < ?
		  AND analyzed = 1 AND pending_save = 0 AND NOT (has_classification = 1 AND stats_extracted = 0)`,
		dbTime(now.AddDate(0, 0, -r.Days)))
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
		if rows.Scan(&o.id, &o.file) == nil && !records[o.id] {
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

// recordSessions restituisce le sessioni che detengono un record di un
// circuito, in gara o in qualifica, in tutti i tornei: gli stessi record delle
// pagine Piste e Statistiche (vedi api.lapRecords).
func (s *Service) recordSessions() (map[int64]bool, error) {
	rows, err := s.db.Query(`
		SELECT s.id, s.track_id, s.session_type
		FROM telemetry_players tp
		JOIN telemetry_sessions s ON s.id = tp.session_id
		JOIN players p ON p.id = tp.player_id
		WHERE tp.best_lap_ms IS NOT NULL AND s.track_id IS NOT NULL AND s.track_id >= 0 AND s.session_type IS NOT NULL
		ORDER BY s.track_id, tp.best_lap_ms, s.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions := map[int64]bool{}
	seen := map[[2]int]bool{} // circuito, 1 = gara o 0 = qualifica
	for rows.Next() {
		var id int64
		var track, sessionType int
		if err := rows.Scan(&id, &track, &sessionType); err != nil {
			return nil, err
		}
		st := telemetry.SessionType(sessionType)
		if !st.IsRace() && !st.IsQualifying() {
			continue // i giri delle prove non sono record
		}
		// Le righe sono in ordine di tempo: la prima per circuito e tipo di sessione è il record.
		key := [2]int{track, 0}
		if st.IsRace() {
			key[1] = 1
		}
		if !seen[key] {
			seen[key] = true
			sessions[id] = true
		}
	}
	return sessions, rows.Err()
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
