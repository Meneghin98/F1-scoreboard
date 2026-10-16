package live

import (
	"database/sql"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"f1-scoreboard/internal/telemetry"
)

var ErrSessionNotFound = errors.New("sessione non trovata")

// SessionRecord è una sessione registrata, come appare nell'elenco.
type SessionRecord struct {
	ID                int64   `json:"id"`
	UID               string  `json:"uid"`
	Format            int     `json:"format"`
	TrackID           *int    `json:"trackId"`
	Track             string  `json:"track"`
	SessionType       *int    `json:"sessionType"`
	Type              string  `json:"type"`
	IsRace            bool    `json:"isRace"`
	TotalLaps         *int    `json:"totalLaps"`
	StartedAt         string  `json:"startedAt"`
	EndedAt           *string `json:"endedAt"`
	Recording         bool    `json:"recording"` // sessione in corso
	Downloadable      bool    `json:"downloadable"`
	SizeBytes         int64   `json:"sizeBytes"`
	Packets           int     `json:"packets"`
	HasClassification bool    `json:"hasClassification"`

	file string
}

// Sessions restituisce le sessioni registrate, dalla più recente.
func (s *Service) Sessions() ([]SessionRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, session_uid, packet_format, track_id, session_type, total_laps, started_at, ended_at,
		       file, size_bytes, packets, has_classification
		FROM telemetry_sessions ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := []SessionRecord{}
	for rows.Next() {
		var r SessionRecord
		err := rows.Scan(&r.ID, &r.UID, &r.Format, &r.TrackID, &r.SessionType, &r.TotalLaps, &r.StartedAt,
			&r.EndedAt, &r.file, &r.SizeBytes, &r.Packets, &r.HasClassification)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	cur := s.current
	var curID int64
	var curPackets int
	if cur != nil {
		curID, curPackets = cur.id, cur.packets
	}
	s.mu.Unlock()

	for i := range records {
		r := &records[i]
		if r.TrackID != nil {
			r.Track = telemetry.TrackID(*r.TrackID).String()
		}
		if r.SessionType != nil {
			st := telemetry.SessionType(*r.SessionType)
			r.Type, r.IsRace = st.String(), st.IsRace()
		}
		if info, err := os.Stat(filepath.Join(s.opts.Dir, r.file)); err == nil {
			r.Downloadable = true
			r.SizeBytes = info.Size()
		}
		if r.ID == curID {
			r.Recording = true
			r.Packets = curPackets
		}
	}
	return records, nil
}

// SessionFile restituisce il percorso della registrazione di una sessione e un
// nome leggibile da proporre per il download.
func (s *Service) SessionFile(id int64) (path, downloadName string, err error) {
	var (
		file, startedAt     string
		trackID, sessionTyp *int
	)
	err = s.db.QueryRow("SELECT file, started_at, track_id, session_type FROM telemetry_sessions WHERE id = ?", id).
		Scan(&file, &startedAt, &trackID, &sessionTyp)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrSessionNotFound
	}
	if err != nil {
		return "", "", err
	}
	path = filepath.Join(s.opts.Dir, file)
	if _, err := os.Stat(path); err != nil {
		return "", "", ErrSessionNotFound
	}

	// Nel database l'orario è in UTC; nel nome del file si usa l'ora locale, come per i file su disco.
	stamp := strings.NewReplacer(" ", "_", ":", "-").Replace(startedAt)
	if t, err := time.Parse("2006-01-02 15:04:05", startedAt); err == nil {
		stamp = t.Local().Format("2006-01-02_15-04-05")
	}
	parts := []string{stamp}
	if trackID != nil {
		parts = append(parts, slug(telemetry.TrackID(*trackID).String()))
	}
	if sessionTyp != nil {
		parts = append(parts, slug(telemetry.SessionType(*sessionTyp).String()))
	}
	return path, strings.Join(parts, "_") + ".f1rec.gz", nil
}

// closeOrphans chiude le sessioni rimaste aperte da un arresto improvviso del server.
func (s *Service) closeOrphans() {
	rows, err := s.db.Query("SELECT id, file, started_at FROM telemetry_sessions WHERE ended_at IS NULL")
	if err != nil {
		log.Printf("telemetria: %v", err)
		return
	}
	type orphan struct {
		id              int64
		file, startedAt string
	}
	var orphans []orphan
	for rows.Next() {
		var o orphan
		if rows.Scan(&o.id, &o.file, &o.startedAt) == nil {
			orphans = append(orphans, o)
		}
	}
	rows.Close()

	for _, o := range orphans {
		var size int64
		end := o.startedAt
		if info, err := os.Stat(filepath.Join(s.opts.Dir, o.file)); err == nil {
			size = info.Size()
			end = dbTime(info.ModTime()) // ultima scrittura su file
		}
		s.exec("UPDATE telemetry_sessions SET ended_at = ?, size_bytes = ? WHERE id = ?", end, size, o.id)
	}
	if len(orphans) > 0 {
		log.Printf("telemetria: chiuse %d sessioni interrotte da un arresto precedente", len(orphans))
	}
}

// slug rende un testo adatto a un nome di file: "Sakhir (Bahrain)" → "sakhir-bahrain".
func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}
