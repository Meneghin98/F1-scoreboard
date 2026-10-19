package live

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"f1-scoreboard/internal/recording"
	"f1-scoreboard/internal/telemetry"
)

const (
	// Senza pacchetti della sessione per questo tempo lo stato diventa "in pausa"
	// (pausa, menù, impostazioni): si continua a mostrare l'ultimo stato ricevuto.
	waitingAfter = 5 * time.Second
	// Senza pacchetti per questo tempo la sessione viene chiusa. Prima della fine
	// della sessione (SEND) contano anche quelli di menù: il gioco è ancora aperto.
	sessionTimeout = 60 * time.Second
	// Con la classifica finale la sessione è conclusa: si chiude appena il gioco
	// smette di mandarla. Sulla schermata dei risultati la ripete ogni 5 secondi,
	// nella prova a tempo la manda una volta sola quando si esce.
	finishedTimeout = 10 * time.Second
	// Neanche i pacchetti di menù tengono aperta una sessione oltre questo tempo
	// senza pacchetti suoi (per esempio se si esce senza finirla).
	menuTimeout = 15 * time.Minute
	// Frequenza massima degli aggiornamenti inviati ai browser.
	broadcastEvery = 250 * time.Millisecond
	// Le sessioni senza classifica finale (abbandonate o interrotte) vengono
	// eliminate dopo questo tempo, a meno che non siano state assegnate a un torneo.
	unfinishedRetention = 48 * time.Hour
	// Ogni quanto si cercano le sessioni non terminate da eliminare.
	cleanupEvery = time.Hour
)

// Options configura il servizio.
type Options struct {
	Dir    string // cartella delle registrazioni
	Record bool   // registra ogni sessione su file
}

// Service riceve la telemetria, tiene lo stato live e registra le sessioni.
type Service struct {
	db   *sql.DB
	opts Options

	mu          sync.Mutex
	tracker     Tracker
	current     *session  // sessione in corso, nil se nessuna
	lastPacket  time.Time // ultimo pacchetto della sessione in corso
	lastAlive   time.Time // ultimo pacchetto qualsiasi, anche di menù: il gioco è aperto
	finishedUID uint64    // ultima sessione chiusa con la classifica finale (vedi Handle)
	warnedFmt   map[uint16]bool
	diag        diagnostics // stato della ricezione, mostrato nella pagina Live
	subscribers map[chan []byte]struct{}
	lastSent    []byte
	stopped     bool // dopo lo spegnimento i pacchetti in ritardo vengono ignorati

	// Torneo in cui finiscono le gare, mostrato nella pagina Live; si rilegge
	// dal database quando targetLoaded è falso.
	target       *LineupTournament
	targetLoaded bool

	// Replay preparati dalle registrazioni, per sessione (vedi replay.go).
	replayMu sync.Mutex
	replays  map[int64]*replayEntry

	done chan struct{}
}

// session è la sessione in corso, con la sua registrazione.
type session struct {
	id             int64 // riga in telemetry_sessions
	uid            uint64
	start          time.Time
	name           string // file della registrazione, relativo a Options.Dir
	file           *os.File
	writer         *recording.Writer
	lastFlush      time.Time
	packets        int
	bytes          int64
	infoSaved      bool
	classification bool
	mapSaved       int     // punti della pianta già salvati nel database
	lineup         *lineup // scelta dei piloti, solo per le gare
	playersSaved   bool    // giocatori già salvati con i piloti scelti: alla fine non si rifà
	ended          bool    // ricevuto l'evento SEND (fine sessione)
}

func NewService(db *sql.DB, opts Options) *Service {
	return &Service{
		db:          db,
		opts:        opts,
		warnedFmt:   map[uint16]bool{},
		subscribers: map[chan []byte]struct{}{},
		replays:     map[int64]*replayEntry{},
		done:        make(chan struct{}),
	}
}

// Done viene chiuso quando Run termina: i client in streaming lo usano per chiudersi.
func (s *Service) Done() <-chan struct{} { return s.done }

// Run ascolta la telemetria su addr finché ctx non viene annullato, poi chiude
// la registrazione in corso.
func (s *Service) Run(ctx context.Context, addr string) {
	defer close(s.done)
	s.closeOrphans()
	s.deleteUnfinished(time.Now()) // prima dell'analisi, così non si leggono file da eliminare
	go s.analyzePending()

	listener := &Listener{Addr: addr, Handle: s.Handle, Ready: s.listening, Failed: s.listenFailed}
	go listener.Run(ctx)

	ticker := time.NewTicker(broadcastEvery)
	defer ticker.Stop()
	cleanup := time.NewTicker(cleanupEvery)
	defer cleanup.Stop()
	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.stopped = true
			s.endSession(time.Now())
			s.mu.Unlock()
			return
		case now := <-cleanup.C:
			s.deleteUnfinished(now)
			// Legge le registrazioni: in background, per non fermare gli aggiornamenti live.
			go s.recoverPending()
		case now := <-ticker.C:
			s.tick(now)
		}
	}
}

// listening registra la porta UDP appena aperta.
func (s *Service) listening(addr net.Addr) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.diag.err = ""
	if udp, ok := addr.(*net.UDPAddr); ok {
		s.diag.port = udp.Port
	}
}

// listenFailed registra perché la porta UDP non si apre: la pagina Live lo mostra.
func (s *Service) listenFailed(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.diag.port, s.diag.err = 0, err.Error()
}

// Handle elabora un pacchetto arrivato all'istante at.
func (s *Service) Handle(at time.Time, data []byte) {
	h, err := telemetry.ParseHeader(data)
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return
	}
	if !telemetry.SupportedFormat(h.PacketFormat) {
		if !s.warnedFmt[h.PacketFormat] {
			s.warnedFmt[h.PacketFormat] = true
			log.Printf("telemetria: formato %d non supportato, nel gioco imposta Formato UDP su 2025 o 2026", h.PacketFormat)
		}
		s.diag.badFormat, s.diag.badFormatAt = h.PacketFormat, at
		return
	}
	s.lastAlive = at
	// SessionUID 0 arriva da menù e caricamenti: non appartiene a nessuna sessione,
	// ma dice che il gioco è ancora aperto.
	if h.SessionUID == 0 {
		return
	}
	// Una sessione chiusa con la classifica finale non si riapre per un suo
	// pacchetto in ritardo (la classifica ripetuta), ma solo se ricomincia da
	// capo con SSTA, come quando si rimanda la registrazione con cmd/record.
	if h.SessionUID == s.finishedUID {
		if code, _ := telemetry.EventCode(data); code != telemetry.EventSessionStarted {
			return
		}
		s.finishedUID = 0
	}
	s.lastPacket = at

	if s.current == nil || s.current.uid != h.SessionUID {
		s.endSession(at)
		s.startSession(at, h)
	}
	cur := s.current
	cur.packets++
	cur.bytes += int64(len(data))
	if cur.writer != nil {
		if err := cur.writer.Write(at.Sub(cur.start), data); err != nil {
			log.Printf("telemetria: scrittura della registrazione: %v", err)
			s.closeFile(cur)
		}
	}

	s.tracker.Handle(h, data)
	if !cur.infoSaved && s.tracker.Session() != nil {
		cur.infoSaved = true
		ses := s.tracker.Session()
		s.exec("UPDATE telemetry_sessions SET track_id = ?, session_type = ?, total_laps = ? WHERE id = ?",
			int(ses.TrackID), int(ses.SessionType), int(ses.TotalLaps), cur.id)
		log.Printf("telemetria: sessione %016x: %s – %s", cur.uid, ses.TrackID, ses.SessionType)
		s.loadTrackMap(cur, ses)
	}
	if !cur.classification && s.tracker.Classification() != nil {
		cur.classification = true
		// Le sessioni del weekend da qui aspettano di essere salvate in un torneo:
		// se il server si spegne prima, al riavvio lo fa recoverPending.
		pending := s.tracker.Session() != nil && sessionKind(s.tracker.Session().SessionType) != ""
		s.exec("UPDATE telemetry_sessions SET has_classification = 1, pending_save = ? WHERE id = ?", pending, cur.id)
		log.Printf("telemetria: sessione %016x: classifica finale ricevuta", cur.uid)
	}
	s.updateLineup(at)

	// SEND arriva alla bandiera a scacchi, prima della classifica finale: la
	// sessione resta aperta, ma da qui i pacchetti di menù non la tengono più in vita.
	if code, ok := telemetry.EventCode(data); ok && code == telemetry.EventSessionEnded {
		cur.ended = true
	}
}

func (s *Service) startSession(at time.Time, h telemetry.Header) {
	name := fmt.Sprintf("%s_%016x.f1rec.gz", at.Format("2006-01-02_15-04-05"), h.SessionUID)
	cur := &session{uid: h.SessionUID, start: at, name: name, lastFlush: at}

	if s.opts.Record {
		if err := s.openFile(cur); err != nil {
			log.Printf("telemetria: impossibile registrare la sessione: %v", err)
		}
	}
	err := s.db.QueryRow(`
		INSERT INTO telemetry_sessions (session_uid, packet_format, started_at, file) VALUES (?, ?, ?, ?)
		RETURNING id`, fmt.Sprintf("%016x", h.SessionUID), h.PacketFormat, dbTime(at), name).Scan(&cur.id)
	if err != nil {
		log.Printf("telemetria: salvataggio della sessione: %v", err)
	}
	s.current = cur
	log.Printf("telemetria: nuova sessione %016x (formato %d)", h.SessionUID, h.PacketFormat)
}

func (s *Service) openFile(cur *session) error {
	if err := os.MkdirAll(s.opts.Dir, 0o755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(s.opts.Dir, cur.name))
	if err != nil {
		return err
	}
	w, err := recording.NewWriter(f)
	if err != nil {
		f.Close()
		return err
	}
	cur.file, cur.writer = f, w
	return nil
}

func (s *Service) closeFile(cur *session) {
	if cur.writer != nil {
		if err := cur.writer.Close(); err != nil {
			log.Printf("telemetria: chiusura della registrazione: %v", err)
		}
		cur.file.Close()
		cur.writer, cur.file = nil, nil
	}
}

// endSession chiude la sessione in corso (se c'è) e ne salva i dati finali.
func (s *Service) endSession(at time.Time) {
	cur := s.current
	if cur == nil {
		return
	}
	s.current = nil
	if cur.classification {
		s.finishedUID = cur.uid
	}
	s.closeFile(cur)
	end := at
	if !s.lastPacket.IsZero() && s.lastPacket.Before(at) {
		end = s.lastPacket
	}
	s.exec("UPDATE telemetry_sessions SET ended_at = ?, packets = ?, size_bytes = ? WHERE id = ?",
		dbTime(end), cur.packets, s.fileSize(cur), cur.id)
	// Il tracker ha ancora i dati di questa sessione: la successiva non è ancora iniziata.
	// Se i risultati aspettavano ancora la conferma, si salvano con i piloti scelti finora.
	// Se non riesce, la sessione resta segnata come da salvare: ci riprova recoverPending.
	if l := cur.lineup; l != nil && l.stage == StageResults {
		if err := s.saveLineup(cur, at); err != nil {
			log.Printf("telemetria: salvataggio della gara: %v", err)
		}
	}
	if !cur.playersSaved {
		d := s.sessionData()
		if cur.lineup != nil {
			d.assigned = cur.lineup.assigned()
		}
		if err := s.savePlayers(cur.id, d); err != nil {
			log.Printf("telemetria: salvataggio dei giocatori: %v", err)
		}
	}
	log.Printf("telemetria: sessione %016x terminata (%d pacchetti)", cur.uid, cur.packets)
}

func (s *Service) fileSize(cur *session) int64 {
	info, err := os.Stat(filepath.Join(s.opts.Dir, cur.name))
	if err != nil {
		return 0
	}
	return info.Size()
}

// tick gestisce le scadenze (scrittura su disco, fine sessione) e invia lo
// stato aggiornato ai browser collegati.
func (s *Service) tick(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cur := s.current; cur != nil {
		// Il gioco ancora aperto (anche solo nei menù) tiene aperta la sessione,
		// finché non è finita (SEND). Dopo la classifica finale conta solo lei.
		alive, timeout := s.lastAlive, sessionTimeout
		switch {
		case cur.classification:
			alive, timeout = s.lastPacket, finishedTimeout
		case cur.ended:
			alive = s.lastPacket
		}
		if now.Sub(alive) > timeout || now.Sub(s.lastPacket) > menuTimeout {
			s.endSession(now)
		} else {
			// Le scadenze della scelta dei piloti valgono anche se il gioco è in pausa.
			s.updateLineup(now)
			if cur.writer != nil && now.Sub(cur.lastFlush) >= time.Second {
				cur.lastFlush = now
				if err := cur.writer.Flush(); err != nil {
					log.Printf("telemetria: scrittura della registrazione: %v", err)
				}
			}
		}
	}

	s.saveTrackMap()

	// Senza browser collegati non serve preparare lo stato: chi si collega lo
	// riceve subito da Subscribe.
	if len(s.subscribers) == 0 {
		s.lastSent = nil
		return
	}
	msg := s.snapshotJSON(now)
	if bytes.Equal(msg, s.lastSent) {
		return
	}
	s.lastSent = msg
	for ch := range s.subscribers {
		send(ch, msg)
	}
}

func (s *Service) snapshotJSON(now time.Time) []byte {
	status := "waiting"
	if s.current != nil {
		status = "paused"
		if now.Sub(s.lastPacket) < waitingAfter {
			status = "live"
		}
	}
	var snap Snapshot
	if s.current != nil {
		snap = s.tracker.Snapshot(status)
		s.lineupView(&snap, now)
	} else {
		snap = emptySnapshot(status)
	}
	snap.Telemetry = s.diag.status(now)
	msg, err := json.Marshal(snap)
	if err != nil {
		log.Printf("telemetria: %v", err)
	}
	return msg
}

// Snapshot restituisce lo stato attuale in JSON.
func (s *Service) Snapshot() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotJSON(time.Now())
}

// loadTrackMap carica la pianta salvata del circuito, se c'è.
func (s *Service) loadTrackMap(cur *session, ses *telemetry.Session) {
	o, err := loadOutline(s.db, ses.TrackID, int(ses.TrackLength))
	if err != nil {
		log.Printf("telemetria: lettura della mappa del circuito: %v", err)
		return
	}
	if o == nil {
		return
	}
	cur.mapSaved = o.filled
	s.tracker.seedOutline(o)
	log.Printf("telemetria: mappa di %s caricata (%d punti)", ses.TrackID, o.filled)
}

// saveTrackMap salva la pianta del circuito quando è completa e ogni volta
// che, da completa, si aggiunge un tratto mancante.
func (s *Service) saveTrackMap() {
	cur, o, ses := s.current, s.tracker.outline, s.tracker.Session()
	if cur == nil || o == nil || ses == nil || !o.complete() || o.filled <= cur.mapSaved {
		return
	}
	if err := saveOutline(s.db, ses.TrackID, o); err != nil {
		log.Printf("telemetria: salvataggio della mappa del circuito: %v", err)
		return
	}
	if cur.mapSaved == 0 {
		log.Printf("telemetria: mappa di %s salvata (%d punti)", ses.TrackID, o.filled)
	}
	cur.mapSaved = o.filled
}

// TrackMap restituisce la pianta del circuito della sessione in corso, o nil.
// È separata dallo stato perché cambia poco ed è molto più grande: il frontend
// la scarica solo quando cambia SessionInfo.MapPoints.
func (s *Service) TrackMap() *TrackMap {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil {
		return nil
	}
	return s.tracker.TrackMap()
}

// Subscribe registra un client che riceve lo stato a ogni cambiamento (subito
// il primo). Il canale contiene sempre solo l'ultimo stato: un client lento
// salta gli aggiornamenti intermedi invece di accumularli.
func (s *Service) Subscribe() (updates <-chan []byte, unsubscribe func()) {
	ch := make(chan []byte, 1)
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	send(ch, s.snapshotJSON(time.Now()))
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		delete(s.subscribers, ch)
		s.mu.Unlock()
	}
}

// send mette msg nel canale sostituendo l'eventuale messaggio non ancora letto.
func send(ch chan []byte, msg []byte) {
	select {
	case <-ch:
	default:
	}
	ch <- msg
}

func (s *Service) exec(query string, args ...any) {
	if _, err := s.db.Exec(query, args...); err != nil {
		log.Printf("telemetria: %v", err)
	}
}

// dbTime formatta un istante come datetime('now') di SQLite (UTC).
func dbTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}
