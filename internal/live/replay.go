package live

import (
	"bytes"
	"compress/flate"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
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
	// Un fotogramma ogni replayKeyEvery si comprime da solo (fotogramma chiave);
	// gli altri usano come dizionario il fotogramma chiave precedente: due stati
	// vicini sono quasi uguali, e così occupano un quarto dello spazio.
	replayKeyEvery = 40
	// Livello di compressione dei fotogrammi: il primo che usa il dizionario
	// (con BestSpeed Go lo ignora), e più veloce dei successivi.
	replayLevel = 2
)

// Replay è una registrazione pronta da riprodurre.
type Replay struct {
	Duration time.Duration
	frames   []replayFrame // in ordine di tempo; si aggiunge un fotogramma solo quando lo stato cambia
	pedals   []PedalSample
	laps     []ReplayLap
	events   []ReplayEvent
	analysis *Analysis
	trackMap *TrackMap
}

type replayFrame struct {
	at time.Duration
	// Snapshot in JSON, compresso con flate (vedi replayKeyEvery): in chiaro
	// un'ora di gara occuperebbe 200 MB.
	data []byte
}

// PedalSample è acceleratore e freno di un giocatore all'istante T (millisecondi dall'inizio).
type PedalSample struct {
	T          int64   `json:"t"`
	Controller int     `json:"controller"` // 1 o 2, come PlayerCar
	Throttle   float32 `json:"throttle"`
	Brake      float32 `json:"brake"`
}

// ReplayLap è l'istante in cui il primo in classifica inizia un giro: sulla
// timeline diventa una tacca.
type ReplayLap struct {
	Lap  int   `json:"lap"`
	AtMS int64 `json:"atMs"`
}

// ReplayEvent è un messaggio della direzione gara con l'istante del replay in cui arriva.
type ReplayEvent struct {
	AtMS int64 `json:"atMs"`
	RaceEvent
}

// ReplayInfo descrive un replay al frontend.
type ReplayInfo struct {
	DurationMS int64         `json:"durationMs"`
	MapPoints  int           `json:"mapPoints"` // punti della pianta completa, 0 se non c'è
	Laps       []ReplayLap   `json:"laps"`
	Events     []ReplayEvent `json:"events"`   // messaggi della direzione gara, per la timeline
	Analysis   *Analysis     `json:"analysis"` // riassunto della sessione alla fine della registrazione, nil se mancano i dati
}

// ReplayFrame è lo stato della sessione a un certo istante del replay.
type ReplayFrame struct {
	T        int64           `json:"t"` // millisecondi dall'inizio
	Snapshot json.RawMessage `json:"snapshot"`
	Pedals   []PedalSample   `json:"pedals"` // ultimi pedalWindow di tutti i giocatori, dal più vecchio
}

func (r *Replay) Info() ReplayInfo {
	info := ReplayInfo{DurationMS: r.Duration.Milliseconds(), Laps: r.laps, Events: r.events, Analysis: r.analysis}
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
	f := ReplayFrame{T: at.Milliseconds(), Snapshot: json.RawMessage(`{"status":"waiting","session":null,"drivers":[],"players":[],"events":[]}`), Pedals: []PedalSample{}}
	// Ultimo fotogramma non successivo ad at.
	if i := sort.Search(len(r.frames), func(i int) bool { return r.frames[i].at > at }); i > 0 {
		data, err := r.frameJSON(i - 1)
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

// frameJSON decomprime il fotogramma i: prima il suo fotogramma chiave, che
// gli fa da dizionario, poi lui.
func (r *Replay) frameJSON(i int) ([]byte, error) {
	k := i - i%replayKeyEvery
	key, err := inflate(r.frames[k].data, nil)
	if err != nil || k == i {
		return key, err
	}
	return inflate(r.frames[i].data, key)
}

// inflate decomprime dati compressi con flate, con il dizionario dict (o senza, se nil).
func inflate(data, dict []byte) ([]byte, error) {
	var zr io.ReadCloser
	if dict == nil {
		zr = flate.NewReader(bytes.NewReader(data))
	} else {
		zr = flate.NewReaderDict(bytes.NewReader(data), dict)
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

// buildReplay ricostruisce gli stati della sessione uid dai pacchetti restituiti
// da next (fino a io.EOF). Con uid 0 vale la prima sessione incontrata. players
// sono i piloti del segnapunti alla guida (indice dell'auto → nome e colore); seed
// restituisce la pianta già salvata del circuito, o nil.
func buildReplay(next func() (recording.Packet, error), uid uint64, players map[int]scoreboardPlayer,
	seed func(*telemetry.Session) *outline) (*Replay, error) {
	var (
		r         = &Replay{laps: []ReplayLap{}, events: []ReplayEvent{}}
		tracker   Tracker
		seeded    bool
		started   bool
		base      time.Duration // arrivo del primo pacchetto della sessione
		last      time.Duration // arrivo dell'ultimo pacchetto, da base
		frameAt   time.Duration // prossimo istante da fotografare
		prev      []byte
		lastLap   int
		lastEvent int // ID dell'ultimo messaggio della direzione gara già sulla timeline
		frames    frameCompressor
		snapshot  = func(at time.Duration) error {
			status := "live"
			if at-last >= waitingAfter {
				status = "paused"
			}
			snap := tracker.Snapshot(status)
			for i := range snap.Drivers {
				p := players[snap.Drivers[i].CarIndex]
				snap.Drivers[i].PlayerName, snap.Drivers[i].PlayerColor = p.Name, p.Color
			}
			if s := snap.Session; s != nil {
				// La pianta si scarica a parte, completa: il numero di punti non deve farla riscaricare.
				s.MapPoints = 0
				if s.IsRace && int(s.CurrentLap) > lastLap {
					lastLap = int(s.CurrentLap)
					r.laps = append(r.laps, ReplayLap{Lap: lastLap, AtMS: at.Milliseconds()})
				}
			}
			if status == "live" {
				for _, p := range snap.Players {
					r.pedals = append(r.pedals, PedalSample{T: at.Milliseconds(), Controller: p.Controller, Throttle: p.Throttle, Brake: p.Brake})
				}
			}
			msg, err := json.Marshal(snap)
			if err != nil {
				return err
			}
			if !bytes.Equal(msg, prev) {
				data, err := frames.compress(msg)
				if err != nil {
					return err
				}
				r.frames = append(r.frames, replayFrame{at: at, data: data})
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
			// Mentre carica la sessione il gioco manda solo eventi (tasti premuti,
			// "sessione iniziata"), senza circuito né piloti: il replay parte
			// quando arrivano gli altri dati.
			if h.PacketID == telemetry.PacketEvent {
				tracker.Handle(h, p.Data)
				continue
			}
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
		for _, e := range tracker.eventsSince(lastEvent) {
			r.events = append(r.events, ReplayEvent{AtMS: at.Milliseconds(), RaceEvent: e})
			lastEvent = e.ID
		}
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
	if a := tracker.Analysis(); a != nil {
		for i := range a.Cars {
			p := players[a.Cars[i].CarIndex]
			a.Cars[i].PlayerName, a.Cars[i].PlayerColor = p.Name, p.Color
		}
		r.analysis = a
	}
	r.trackMap = tracker.TrackMap()
	if r.trackMap != nil && len(r.trackMap.Points) < 2 {
		r.trackMap = nil
	}
	return r, nil
}

// frameCompressor comprime i fotogrammi del replay nell'ordine in cui
// arrivano: il primo di ogni gruppo di replayKeyEvery da solo, gli altri con
// lui come dizionario (vedi Replay.frameJSON per l'operazione inversa).
type frameCompressor struct {
	n     int           // fotogrammi compressi finora
	key   *flate.Writer // per i fotogrammi chiave
	delta *flate.Writer // per gli altri: ha come dizionario l'ultimo fotogramma chiave
}

func (c *frameCompressor) compress(msg []byte) ([]byte, error) {
	var err error
	w := c.delta
	if c.n%replayKeyEvery == 0 {
		if c.key == nil {
			c.key, err = flate.NewWriter(nil, replayLevel)
		}
		w = c.key
		if err == nil {
			// Il dizionario si sceglie solo creando il compressore: ne serve uno nuovo per ogni gruppo.
			c.delta, err = flate.NewWriterDict(nil, replayLevel, msg)
		}
		if err != nil {
			return nil, err
		}
	}
	c.n++
	var buf bytes.Buffer
	w.Reset(&buf)
	w.Write(msg)
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

// loadReplay prepara il replay della sessione id leggendo la registrazione,
// o lo rilegge dal file in cui è stato salvato, se vale ancora.
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

	players := map[int]scoreboardPlayer{}
	rows, err := s.db.Query(`
		SELECT tp.car_index, p.name, COALESCE(p.color, '') FROM telemetry_players tp JOIN players p ON p.id = tp.player_id
		WHERE tp.session_id = ?`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var car int
		var p scoreboardPlayer
		if err := rows.Scan(&car, &p.Name, &p.Color); err != nil {
			rows.Close()
			return nil, err
		}
		players[car] = p
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	path := filepath.Join(s.opts.Dir, file)
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}
	// Il replay salvato vale se è dello stesso eseguibile, della stessa
	// registrazione e con gli stessi piloti (vedi replaycache.go).
	header := replayHeader{Executable: executableID(), Recording: info.Size(), Players: players}
	cache := path + replayExt
	cached := func() *Replay {
		if header.Executable == "" {
			return nil
		}
		return readReplay(cache, header)
	}
	if r := cached(); r != nil {
		return r, nil
	}

	// Un replay alla volta: è un lavoro pesante. Se intanto lo stesso replay è
	// stato preparato (a fine sessione, in background) lo si rilegge dal file.
	s.replayBuild <- struct{}{}
	defer func() { <-s.replayBuild }()
	if r := cached(); r != nil {
		return r, nil
	}
	start := time.Now()
	r, err := s.buildFromFile(path, uid, players)
	if err != nil {
		return nil, err
	}
	log.Printf("telemetria: replay della sessione %d preparato in %s", id, time.Since(start).Round(100*time.Millisecond))
	if header.Executable != "" {
		if err := saveReplay(cache, header, r); err != nil {
			// Non è grave: la prossima volta si prepara di nuovo.
			log.Printf("telemetria: salvataggio del replay della sessione %d: %v", id, err)
		}
	}
	return r, nil
}

// buildFromFile prepara il replay della sessione uid leggendo la registrazione in path.
func (s *Service) buildFromFile(path string, uid uint64, players map[int]scoreboardPlayer) (*Replay, error) {
	f, err := os.Open(path)
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
	// Se il server si sta spegnendo la lettura si interrompe, invece di farlo aspettare.
	next := func() (recording.Packet, error) {
		select {
		case <-s.quit:
			return recording.Packet{}, errStopping
		default:
			return rec.Next()
		}
	}
	seed := func(ses *telemetry.Session) *outline {
		o, err := loadOutline(s.db, ses.TrackID, int(ses.TrackLength))
		if err != nil {
			return nil // senza la pianta salvata si usa quella ricostruita dalla registrazione
		}
		return o
	}
	return buildReplay(next, uid, players, seed)
}
