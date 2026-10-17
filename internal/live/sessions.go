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
	Cars              *int    `json:"cars"` // auto in pista, nil se non ancora noto

	// Analyzed è falso finché i giocatori non sono stati estratti dalla registrazione.
	Analyzed bool            `json:"analyzed"`
	Players  []SessionPlayer `json:"players"`
	Race     *SessionRace    `json:"race"` // gara del torneo a cui appartiene, nil se nessuna
	// Kind è "race", "qualifying" o "practice" per le sessioni del weekend, "" per le altre.
	Kind string `json:"kind"`
	// ProvidesResults è vero per le sessioni del weekend con la classifica finale:
	// i risultati della gara del torneo a cui sono collegate vengono presi da qui.
	ProvidesResults bool `json:"providesResults"`
	// DeleteAt è quando la sessione verrà eliminata perché non terminata (UTC), nil se resta.
	DeleteAt *string `json:"deleteAt"`

	file string
}

// SessionPlayer è un giocatore umano della sessione.
type SessionPlayer struct {
	CarIndex   int     `json:"carIndex"`
	Name       string  `json:"name"` // nome mostrato dal gioco
	Team       string  `json:"team"`
	RaceNumber int     `json:"raceNumber"`
	Position   *int    `json:"position"` // posizione finale, nil senza classifica
	DNF        bool    `json:"dnf"`
	PlayerID   *int64  `json:"playerId"` // pilota del segnapunti associato
	PlayerName *string `json:"playerName"`
}

// SessionRace è la gara del torneo collegata a una sessione.
type SessionRace struct {
	ID             int64   `json:"id"`
	Track          string  `json:"track"`
	Type           string  `json:"type"`
	Date           *string `json:"date"`
	TournamentID   int64   `json:"tournamentId"`
	TournamentName string  `json:"tournamentName"`
}

// Sessions restituisce le sessioni registrate, dalla più recente.
func (s *Service) Sessions() ([]SessionRecord, error) {
	rows, err := s.db.Query(`
		SELECT s.id, s.session_uid, s.packet_format, s.track_id, s.session_type, s.total_laps, s.started_at, s.ended_at,
		       s.file, s.size_bytes, s.packets, s.has_classification, s.cars, s.analyzed,
		       r.id, r.track, r.type, r.date, t.id, t.name
		FROM telemetry_sessions s
		LEFT JOIN races r ON r.id = s.race_id
		LEFT JOIN tournaments t ON t.id = r.tournament_id
		ORDER BY s.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := []SessionRecord{}
	byID := map[int64]int{} // posizione in records, per aggiungere i giocatori
	for rows.Next() {
		var (
			r                     SessionRecord
			raceID, tournamentID  *int64
			raceTrack, raceType   *string
			raceDate, tournamentN *string
		)
		err := rows.Scan(&r.ID, &r.UID, &r.Format, &r.TrackID, &r.SessionType, &r.TotalLaps, &r.StartedAt,
			&r.EndedAt, &r.file, &r.SizeBytes, &r.Packets, &r.HasClassification, &r.Cars, &r.Analyzed,
			&raceID, &raceTrack, &raceType, &raceDate, &tournamentID, &tournamentN)
		if err != nil {
			return nil, err
		}
		if raceID != nil {
			r.Race = &SessionRace{ID: *raceID, Track: *raceTrack, Type: *raceType, Date: raceDate,
				TournamentID: *tournamentID, TournamentName: *tournamentN}
		}
		r.Players = []SessionPlayer{}
		byID[r.ID] = len(records)
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.addPlayers(records, byID); err != nil {
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
			r.Type, r.IsRace, r.Kind = st.String(), st.IsRace(), sessionKind(st)
		}
		r.ProvidesResults = r.Kind != "" && r.HasClassification
		if !r.HasClassification && r.Race == nil && r.EndedAt != nil {
			if end, err := time.Parse("2006-01-02 15:04:05", *r.EndedAt); err == nil {
				at := end.Add(unfinishedRetention).Format("2006-01-02 15:04:05")
				r.DeleteAt = &at
			}
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

// addPlayers aggiunge alle sessioni i loro giocatori, in ordine d'arrivo.
func (s *Service) addPlayers(records []SessionRecord, byID map[int64]int) error {
	rows, err := s.db.Query(`
		SELECT tp.session_id, tp.car_index, tp.name, tp.team, tp.race_number, tp.position, tp.result_status,
		       tp.player_id, p.name
		FROM telemetry_players tp LEFT JOIN players p ON p.id = tp.player_id
		ORDER BY tp.session_id, tp.position IS NULL, tp.position, tp.car_index`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			sessionID int64
			p         SessionPlayer
			status    *int
		)
		err := rows.Scan(&sessionID, &p.CarIndex, &p.Name, &p.Team, &p.RaceNumber, &p.Position, &status,
			&p.PlayerID, &p.PlayerName)
		if err != nil {
			return err
		}
		p.DNF = status != nil && !classified(status)
		if i, ok := byID[sessionID]; ok {
			records[i].Players = append(records[i].Players, p)
		}
	}
	return rows.Err()
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

// raceTrackNames sono i nomi dei circuiti come li propone il frontend per le
// gare inserite a mano (web/src/lib/f1.ts): una gara creata da una
// registrazione si chiama come una inserita a mano.
var raceTrackNames = map[telemetry.TrackID]string{
	0: "Australia – Melbourne", 2: "Cina – Shanghai", 3: "Bahrain – Sakhir", 4: "Spagna – Barcellona",
	5: "Monaco", 6: "Canada – Montréal", 7: "Gran Bretagna – Silverstone", 9: "Ungheria – Hungaroring",
	10: "Belgio – Spa-Francorchamps", 11: "Italia – Monza", 12: "Singapore – Marina Bay", 13: "Giappone – Suzuka",
	14: "Abu Dhabi – Yas Marina", 15: "USA – Austin", 16: "Brasile – Interlagos", 17: "Austria – Red Bull Ring",
	19: "Messico – Città del Messico", 20: "Azerbaijan – Baku", 26: "Olanda – Zandvoort",
	27: "Emilia-Romagna – Imola", 29: "Arabia Saudita – Jeddah", 30: "Miami", 31: "Las Vegas", 32: "Qatar – Lusail",
}

// RaceTrackName restituisce il nome del circuito da usare per una gara del torneo.
func RaceTrackName(id telemetry.TrackID) string {
	if name, ok := raceTrackNames[id]; ok {
		return name
	}
	return id.String()
}
