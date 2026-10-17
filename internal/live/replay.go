package live

import (
	"bytes"
	"compress/flate"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"f1-scoreboard/internal/recording"
	"f1-scoreboard/internal/telemetry"
)

// Il replay rilegge una registrazione e la mostra come la pagina Live. La
// registrazione viene letta una volta sola e trasformata nella sequenza degli
// stati che il browser avrebbe ricevuto in diretta (uno ogni broadcastEvery):
// spostarsi avanti e indietro è poi solo una ricerca in un elenco.
const (
	// Storico di acceleratore e freno restituito con ogni fotogramma, come nella pagina Live.
	pedalWindow = 20 * time.Second
	// I replay restano in memoria finché qualcuno li guarda: dopo questo tempo senza richieste si liberano.
	replayIdle = 10 * time.Minute
	// Replay tenuti in memoria al massimo: oltre si libera quello usato meno di recente.
	replayCacheSize = 2
)

// Replay è una registrazione pronta da riprodurre.
type Replay struct {
	Duration time.Duration
	frames   []replayFrame // in ordine di tempo; si aggiunge un fotogramma solo quando lo stato cambia
	pedals   []PedalSample
	laps     []ReplayLap
	trackMap *TrackMap
}

type replayFrame struct {
	at   time.Duration
	data []byte // Snapshot in JSON, compresso con flate: in chiaro un'ora di gara occuperebbe 100 MB
}

// PedalSample è acceleratore e freno del giocatore all'istante T (millisecondi dall'inizio).
type PedalSample struct {
	T        int64   `json:"t"`
	Throttle float32 `json:"throttle"`
	Brake    float32 `json:"brake"`
}

// ReplayLap è l'istante in cui il primo in classifica inizia un giro: sulla
// timeline diventa una tacca.
type ReplayLap struct {
	Lap  int   `json:"lap"`
	AtMS int64 `json:"atMs"`
}

// ReplayInfo descrive un replay al frontend.
type ReplayInfo struct {
	DurationMS int64       `json:"durationMs"`
	MapPoints  int         `json:"mapPoints"` // punti della pianta completa, 0 se non c'è
	Laps       []ReplayLap `json:"laps"`
}

// ReplayFrame è lo stato della sessione a un certo istante del replay.
type ReplayFrame struct {
	T        int64           `json:"t"` // millisecondi dall'inizio
	Snapshot json.RawMessage `json:"snapshot"`
	Pedals   []PedalSample   `json:"pedals"` // ultimi pedalWindow, dal più vecchio
}

func (r *Replay) Info() ReplayInfo {
	info := ReplayInfo{DurationMS: r.Duration.Milliseconds(), Laps: r.laps}
	if r.trackMap != nil {
		info.MapPoints = len(r.trackMap.Points)
	}
	return info
}

// TrackMap restituisce la pianta del circuito com'era alla fine della
// registrazione: nel replay la mappa è completa fin dall'inizio. Nil se non c'è.
func (r *Replay) TrackMap() *TrackMap { return r.trackMap }

// Frame restituisce lo stato all'istante at (limitato alla durata del replay).
func (r *Replay) Frame(at time.Duration) (ReplayFrame, error) {
	at = min(max(at, 0), r.Duration)
	f := ReplayFrame{T: at.Milliseconds(), Snapshot: json.RawMessage(`{"status":"waiting","session":null,"drivers":[]}`), Pedals: []PedalSample{}}
	// Ultimo fotogramma non successivo ad at.
	if i := sort.Search(len(r.frames), func(i int) bool { return r.frames[i].at > at }); i > 0 {
		data, err := io.ReadAll(flate.NewReader(bytes.NewReader(r.frames[i-1].data)))
		if err != nil {
			return f, err
		}
		f.Snapshot = data
	}
	from := (at - pedalWindow).Milliseconds()
	lo := sort.Search(len(r.pedals), func(i int) bool { return r.pedals[i].T >= from })
	hi := sort.Search(len(r.pedals), func(i int) bool { return r.pedals[i].T > f.T })
	if lo < hi {
		f.Pedals = r.pedals[lo:hi]
	}
	return f, nil
}

// buildReplay ricostruisce gli stati della sessione uid dai pacchetti restituiti
// da next (fino a io.EOF). Con uid 0 vale la prima sessione incontrata. names
// sono i piloti del segnapunti alla guida (indice dell'auto → nome); seed
// restituisce la pianta già salvata del circuito, o nil.
func buildReplay(next func() (recording.Packet, error), uid uint64, names map[int]string,
	seed func(*telemetry.Session) *outline) (*Replay, error) {
	var (
		r        = &Replay{laps: []ReplayLap{}}
		tracker  Tracker
		seeded   bool
		started  bool
		base     time.Duration // arrivo del primo pacchetto della sessione
		last     time.Duration // arrivo dell'ultimo pacchetto, da base
		frameAt  time.Duration // prossimo istante da fotografare
		prev     []byte
		lastLap  int
		zw, _    = flate.NewWriter(nil, flate.BestSpeed)
		snapshot = func(at time.Duration) error {
			status := "live"
			if at-last >= waitingAfter {
				status = "waiting"
			}
			snap := tracker.Snapshot(status)
			for i := range snap.Drivers {
				snap.Drivers[i].PlayerName = names[snap.Drivers[i].CarIndex]
			}
			if s := snap.Session; s != nil {
				// La pianta si scarica a parte, completa: il numero di punti non deve farla riscaricare.
				s.MapPoints = 0
				if s.IsRace && int(s.CurrentLap) > lastLap {
					lastLap = int(s.CurrentLap)
					r.laps = append(r.laps, ReplayLap{Lap: lastLap, AtMS: at.Milliseconds()})
				}
			}
			if p := snap.Player; p != nil && status == "live" {
				r.pedals = append(r.pedals, PedalSample{T: at.Milliseconds(), Throttle: p.Throttle, Brake: p.Brake})
			}
			msg, err := json.Marshal(snap)
			if err != nil {
				return err
			}
			if !bytes.Equal(msg, prev) {
				var buf bytes.Buffer
				zw.Reset(&buf)
				zw.Write(msg)
				if err := zw.Close(); err != nil {
					return err
				}
				r.frames = append(r.frames, replayFrame{at: at, data: buf.Bytes()})
				prev = msg
			}
			return nil
		}
	)

	for {
		p, err := next()
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			break // un file troncato si riproduce fin dove è valido
		}
		if err != nil {
			return nil, err
		}
		h, err := telemetry.ParseHeader(p.Data)
		if err != nil || h.SessionUID == 0 || !telemetry.SupportedFormat(h.PacketFormat) {
			continue
		}
		if uid == 0 {
			uid = h.SessionUID
		}
		if h.SessionUID != uid {
			continue
		}
		if !started {
			// Il replay parte dal primo pacchetto della sessione.
			started, base = true, p.At
		}
		at := p.At - base
		// Gli istanti prima di questo pacchetto mostrano lo stato precedente.
		for ; frameAt < at; frameAt += broadcastEvery {
			if err := snapshot(frameAt); err != nil {
				return nil, err
			}
		}
		tracker.Handle(h, p.Data)
		last = at
		if !seeded && tracker.Session() != nil {
			seeded = true
			if o := seed(tracker.Session()); o != nil {
				tracker.seedOutline(o)
			}
		}
	}
	if !started {
		return nil, errors.New("la registrazione non contiene dati della sessione")
	}
	if err := snapshot(last); err != nil {
		return nil, err
	}
	r.Duration = last
	r.trackMap = tracker.TrackMap()
	if r.trackMap != nil && len(r.trackMap.Points) < 2 {
		r.trackMap = nil
	}
	return r, nil
}

// --- Replay in memoria -------------------------------------------------------

type replayEntry struct {
	ready  chan struct{} // chiuso quando replay ed err sono pronti
	replay *Replay
	err    error
	used   time.Time
}

// Replay restituisce il replay della sessione id, preparandolo se non è già in
// memoria. La prima richiesta legge tutta la registrazione e può richiedere
// qualche secondo; le richieste contemporanee aspettano la stessa lettura.
func (s *Service) Replay(id int64) (*Replay, error) {
	now := time.Now()
	s.replayMu.Lock()
	s.evictReplays(now)
	e := s.replays[id]
	if e == nil {
		e = &replayEntry{ready: make(chan struct{})}
		s.replays[id] = e
		go func() {
			e.replay, e.err = s.loadReplay(id)
			close(e.ready)
		}()
	}
	e.used = now
	s.replayMu.Unlock()

	<-e.ready
	if e.err != nil {
		// Un errore non resta in memoria: alla prossima richiesta si riprova.
		s.replayMu.Lock()
		if s.replays[id] == e {
			delete(s.replays, id)
		}
		s.replayMu.Unlock()
	}
	return e.replay, e.err
}

// evictReplays libera i replay non usati da replayIdle e, se sono troppi,
// quelli usati meno di recente. Va chiamata con replayMu bloccato.
func (s *Service) evictReplays(now time.Time) {
	for id, e := range s.replays {
		if now.Sub(e.used) > replayIdle {
			delete(s.replays, id)
		}
	}
	for len(s.replays) >= replayCacheSize {
		var oldest int64
		var oldestUsed time.Time
		for id, e := range s.replays {
			if oldestUsed.IsZero() || e.used.Before(oldestUsed) {
				oldest, oldestUsed = id, e.used
			}
		}
		delete(s.replays, oldest)
	}
}

// forgetReplay toglie dalla memoria il replay di una sessione eliminata o modificata.
func (s *Service) forgetReplay(id int64) {
	s.replayMu.Lock()
	delete(s.replays, id)
	s.replayMu.Unlock()
}

func (s *Service) loadReplay(id int64) (*Replay, error) {
	var file, uidHex string
	err := s.db.QueryRow("SELECT file, session_uid FROM telemetry_sessions WHERE id = ?", id).Scan(&file, &uidHex)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	inProgress := s.current != nil && s.current.id == id
	s.mu.Unlock()
	if inProgress {
		return nil, ErrSessionRecording
	}
	uid, err := strconv.ParseUint(uidHex, 16, 64)
	if err != nil {
		return nil, fmt.Errorf("sessione %d: uid %q non valido", id, uidHex)
	}

	names := map[int]string{}
	rows, err := s.db.Query(`
		SELECT tp.car_index, p.name FROM telemetry_players tp JOIN players p ON p.id = tp.player_id
		WHERE tp.session_id = ?`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var car int
		var name string
		if err := rows.Scan(&car, &name); err != nil {
			rows.Close()
			return nil, err
		}
		names[car] = name
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	f, err := os.Open(filepath.Join(s.opts.Dir, file))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rec, err := recording.NewReader(f)
	if err != nil {
		return nil, err
	}
	seed := func(ses *telemetry.Session) *outline {
		o, err := loadOutline(s.db, ses.TrackID, int(ses.TrackLength))
		if err != nil {
			return nil // senza la pianta salvata si usa quella ricostruita dalla registrazione
		}
		return o
	}
	return buildReplay(rec.Next, uid, names, seed)
}
