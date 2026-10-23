package live

import (
	"database/sql"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Riavvii e flashback di una sessione, mostrati nella pagina Gare.
//
// Il flashback ha un evento suo (FLBK): basta contarli. Il riavvio no: il gioco
// chiude la sessione e ne apre un'altra con un SessionUID nuovo, pochi secondi
// dopo. Si riconosce dalla sessione precedente: stesso circuito e stesso tipo,
// senza classifica finale e finita da poco. Chi esce al menù e rientra subito
// sulla stessa pista conta come un riavvio: dalla telemetria non si distingue.

// restartGap è il tempo massimo tra la fine di una sessione e l'inizio della
// successiva perché sia un riavvio (nelle registrazioni sono pochi secondi).
const restartGap = 2 * time.Minute

// restartsOf calcola i riavvii della sessione id: quelli della precedente più
// uno, se la precedente è stata ricominciata con questa.
func restartsOf(db dbtx, id int64) (int, error) {
	var (
		uid, startedAt       string
		trackID, sessionType *int
	)
	err := db.QueryRow("SELECT session_uid, started_at, track_id, session_type FROM telemetry_sessions WHERE id = ?", id).
		Scan(&uid, &startedAt, &trackID, &sessionType)
	if err != nil {
		return 0, err
	}
	var (
		prevUID             string
		prevTrack, prevType *int
		prevClassification  bool
		prevEnded           *string
		prevRestarts        *int
	)
	err = db.QueryRow(`
		SELECT session_uid, track_id, session_type, has_classification, ended_at, restarts
		FROM telemetry_sessions WHERE id < ? ORDER BY id DESC LIMIT 1`, id).
		Scan(&prevUID, &prevTrack, &prevType, &prevClassification, &prevEnded, &prevRestarts)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if prevClassification {
		return 0, nil // la precedente è finita: questa è una sessione nuova
	}
	n := 0
	if prevRestarts != nil {
		n = *prevRestarts
	}
	// Stessa sessione del gioco su due righe: il server è ripartito a sessione in corso.
	if prevUID == uid {
		return n, nil
	}
	if trackID == nil || sessionType == nil || prevTrack == nil || prevType == nil || prevEnded == nil ||
		*trackID != *prevTrack || *sessionType != *prevType {
		return 0, nil
	}
	start, err1 := time.Parse("2006-01-02 15:04:05", startedAt)
	end, err2 := time.Parse("2006-01-02 15:04:05", *prevEnded)
	if err1 != nil || err2 != nil || start.Sub(end) > restartGap {
		return 0, nil
	}
	return n + 1, nil
}

// saveRestarts calcola e salva i riavvii della sessione id.
func (s *Service) saveRestarts(id int64) {
	n, err := restartsOf(s.db, id)
	if err != nil {
		log.Printf("telemetria: riavvii della sessione %d: %v", id, err)
		return
	}
	s.exec("UPDATE telemetry_sessions SET restarts = ? WHERE id = ?", n, id)
}

// backfillCounters conta riavvii e flashback delle sessioni registrate prima
// che il server lo facesse da solo, o interrotte da uno spegnimento improvviso.
// I flashback si rileggono dal file: senza file restano da contare.
func (s *Service) backfillCounters() {
	// In ordine: i riavvii di una sessione partono da quelli della precedente.
	ids, err := s.pendingIDs("SELECT id FROM telemetry_sessions WHERE restarts IS NULL AND ended_at IS NOT NULL ORDER BY id")
	if err != nil {
		log.Printf("telemetria: %v", err)
		return
	}
	for _, id := range ids {
		s.saveRestarts(id)
	}

	type pending struct {
		id        int64
		uid, file string
	}
	rows, err := s.db.Query("SELECT id, session_uid, file FROM telemetry_sessions WHERE flashbacks IS NULL AND ended_at IS NOT NULL")
	if err != nil {
		log.Printf("telemetria: %v", err)
		return
	}
	var list []pending
	for rows.Next() {
		var p pending
		if rows.Scan(&p.id, &p.uid, &p.file) == nil {
			list = append(list, p)
		}
	}
	rows.Close()
	counted := 0
	for _, p := range list {
		uid, err := strconv.ParseUint(p.uid, 16, 64)
		if err != nil {
			continue
		}
		d, err := analyzeFile(filepath.Join(s.opts.Dir, p.file), uid)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				log.Printf("telemetria: lettura della sessione %d: %v", p.id, err)
			}
			continue
		}
		s.exec("UPDATE telemetry_sessions SET flashbacks = ? WHERE id = ?", d.flashbacks, p.id)
		counted++
	}
	if counted > 0 {
		log.Printf("telemetria: contati i flashback di %d sessioni registrate", counted)
	}
}

// pendingIDs restituisce gli id scelti da query.
func (s *Service) pendingIDs(query string) ([]int64, error) {
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
