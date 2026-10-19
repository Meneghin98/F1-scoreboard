package live

import (
	"database/sql"
	"errors"
)

// Le gare dei tornei possono venire dalle registrazioni. Qui ci sono le
// operazioni sulle gare che devono tenerne conto: ogni sessione finita sta
// sempre in un torneo, e i risultati presi da una registrazione si possono
// correggere a mano senza che la registrazione li riscriva.

// ErrRaceNotFound indica una gara che non esiste.
var ErrRaceNotFound = errors.New("gara non trovata")

// plannedRace cerca nel torneo una gara in calendario per la sessione: stesso
// circuito e stesso tipo, ancora senza risultati né registrazioni. Le gare in
// calendario si riempiono nell'ordine delle date. 0 se non ce n'è una.
func plannedRace(tx dbtx, tournamentID int64, spec raceSpec) (int64, error) {
	var id int64
	err := tx.QueryRow(`
		SELECT r.id FROM races r
		WHERE r.tournament_id = ? AND r.type = ?
		  AND (r.track_id = ? OR (r.track_id IS NULL AND r.track = ? COLLATE NOCASE))
		  AND NOT EXISTS (SELECT 1 FROM results WHERE race_id = r.id)
		  AND NOT EXISTS (SELECT 1 FROM telemetry_sessions WHERE race_id = r.id)
		ORDER BY r.date IS NULL, r.date, r.id LIMIT 1`,
		tournamentID, spec.kind, spec.trackID, spec.track).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// RaceRecording è una registrazione collegata a una gara del torneo.
type RaceRecording struct {
	ID     int64 `json:"id"`
	Source bool  `json:"source"` // i risultati della gara vengono da questa registrazione
}

// TournamentRecordings restituisce le registrazioni collegate alle gare del
// torneo, per gara, dalla più recente.
func (s *Service) TournamentRecordings(tournamentID int64) (map[int64][]RaceRecording, error) {
	rows, err := s.db.Query(`
		SELECT r.id, s.id, COALESCE(`+sourceSQL+`, 0)
		FROM telemetry_sessions s JOIN races r ON r.id = s.race_id
		WHERE r.tournament_id = ? ORDER BY s.id DESC`, tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]RaceRecording{}
	seenSource := map[int64]bool{} // la fonte dei risultati è la più recente (vedi resultsSource)
	for rows.Next() {
		var raceID int64
		var rec RaceRecording
		if err := rows.Scan(&raceID, &rec.ID, &rec.Source); err != nil {
			return nil, err
		}
		if rec.Source {
			rec.Source, seenSource[raceID] = !seenSource[raceID], true
		}
		out[raceID] = append(out[raceID], rec)
	}
	return out, rows.Err()
}

// ResyncRace ripristina i risultati della gara dalla sua registrazione,
// annullando le modifiche fatte a mano.
func (s *Service) ResyncRace(raceID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	source, err := resultsSource(tx, raceID)
	if err != nil {
		return err
	}
	if source == 0 {
		return invalid("La gara non ha una registrazione con la classifica finale da cui prendere i risultati")
	}
	if _, err := tx.Exec("UPDATE races SET manual_results = 0 WHERE id = ?", raceID); err != nil {
		return err
	}
	if err := syncRaceResults(tx, raceID); err != nil {
		return err
	}
	return tx.Commit()
}

// CheckRaceType verifica che una gara possa diventare del tipo indicato: se
// i risultati vengono da una registrazione, il tipo deve restare dello stesso
// gruppo (una qualifica non dà i risultati di un Gran Premio).
func (s *Service) CheckRaceType(raceID int64, raceType string) error {
	var kind *string
	err := s.db.QueryRow(`
		SELECT `+sessionKindSQL+` FROM telemetry_sessions s JOIN races r ON r.id = s.race_id
		WHERE r.id = ? AND `+sourceSQL+` ORDER BY s.id DESC LIMIT 1`, raceID).Scan(&kind)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	newKind := raceType
	if raceType == "sprint" {
		newKind = "race"
	}
	if kind != nil && *kind != newKind {
		return invalid("I risultati vengono da una registrazione di tipo diverso: sposta prima la registrazione in un'altra gara")
	}
	return nil
}

// DeleteRace elimina una gara con i suoi risultati. Le sue registrazioni
// passano in "Nessun torneo", a meno che la gara non fosse già lì: in quel
// caso restano senza torneo. Restituisce quante registrazioni sono state spostate.
func (s *Service) DeleteRace(raceID int64) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var free bool
	err = tx.QueryRow("SELECT t.free FROM races r JOIN tournaments t ON t.id = r.tournament_id WHERE r.id = ?", raceID).Scan(&free)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrRaceNotFound
	}
	if err != nil {
		return 0, err
	}
	var moved []linkedSession
	if !free {
		if moved, err = linkedSessions(tx, "r.id = ?", raceID); err != nil {
			return 0, err
		}
	}
	// Le registrazioni restano senza gara (ON DELETE SET NULL); poi si spostano.
	if _, err := tx.Exec("DELETE FROM races WHERE id = ?", raceID); err != nil {
		return 0, err
	}
	if err := moveToFree(tx, moved); err != nil {
		return 0, err
	}
	return len(moved), tx.Commit()
}

// DeleteTournament elimina un torneo con le sue gare. Le registrazioni delle
// gare passano in "Nessun torneo": restano in un torneo, come tutte le sessioni
// finite. Restituisce quante registrazioni sono state spostate.
func (s *Service) DeleteTournament(tournamentID int64) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var free bool
	err = tx.QueryRow("SELECT free FROM tournaments WHERE id = ?", tournamentID).Scan(&free)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, invalid("Torneo inesistente")
	}
	if err != nil {
		return 0, err
	}
	if free {
		return 0, invalid("\"Nessun torneo\" non si può eliminare")
	}
	moved, err := linkedSessions(tx, "r.tournament_id = ?", tournamentID)
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec("DELETE FROM tournaments WHERE id = ?", tournamentID); err != nil {
		return 0, err
	}
	if err := moveToFree(tx, moved); err != nil {
		return 0, err
	}
	return len(moved), tx.Commit()
}

// linkedSession è una registrazione di una gara che sta per essere eliminata,
// con il tipo della gara (per ricrearla uguale in "Nessun torneo").
type linkedSession struct {
	id       int64
	raceType string
}

// linkedSessions restituisce le registrazioni delle gare scelte da where (con alias r).
func linkedSessions(tx dbtx, where string, arg int64) ([]linkedSession, error) {
	rows, err := tx.Query("SELECT s.id, r.type FROM telemetry_sessions s JOIN races r ON r.id = s.race_id WHERE "+where+" ORDER BY s.id", arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []linkedSession
	for rows.Next() {
		var l linkedSession
		if err := rows.Scan(&l.id, &l.raceType); err != nil {
			return nil, err
		}
		list = append(list, l)
	}
	return list, rows.Err()
}

// moveToFree mette ogni registrazione in una nuova gara di "Nessun torneo".
func moveToFree(tx dbtx, list []linkedSession) error {
	if len(list) == 0 {
		return nil
	}
	var free int64
	if err := tx.QueryRow("SELECT id FROM tournaments WHERE free = 1").Scan(&free); err != nil {
		return err
	}
	for _, d := range list {
		if _, err := linkNewRace(tx, d.id, free, d.raceType); err != nil {
			return err
		}
	}
	return nil
}
