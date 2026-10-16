// Comando record: registra su file i pacchetti UDP della telemetria di F1 25.
//
//	go run ./cmd/record                  registra in recordings/<data-ora>.f1rec.gz finché non premi Ctrl+C
//	go run ./cmd/record -out gara.f1rec.gz
//	go run ./cmd/record -info gara.f1rec.gz   riassume il contenuto di una registrazione
//	go run ./cmd/record -replay gara.f1rec.gz [-speed 4] [-to 127.0.0.1:20777]
//	                                         la rimanda via UDP, come se fosse il gioco in diretta
//
// Nel gioco: Impostazioni → Impostazioni telemetria → Telemetria UDP attiva, porta 20777,
// formato 2025 (o 2026 con il DLC 2026 Season Pack), IP di questo PC (127.0.0.1 se il gioco gira qui).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"f1-scoreboard/internal/recording"
	"f1-scoreboard/internal/telemetry"
)

// Descrizioni dei codici evento.
var eventNames = map[string]string{
	telemetry.EventSessionStarted:      "inizio sessione",
	telemetry.EventSessionEnded:        "fine sessione",
	telemetry.EventFastestLap:          "giro più veloce",
	telemetry.EventRetirement:          "ritiro",
	telemetry.EventDRSEnabled:          "DRS abilitato",
	telemetry.EventDRSDisabled:         "DRS disabilitato",
	telemetry.EventTeamMateInPits:      "compagno ai box",
	telemetry.EventChequeredFlag:       "bandiera a scacchi",
	telemetry.EventRaceWinner:          "vincitore della gara",
	telemetry.EventPenalty:             "penalità",
	telemetry.EventSpeedTrap:           "speed trap",
	telemetry.EventStartLights:         "semaforo di partenza",
	telemetry.EventLightsOut:           "semaforo spento",
	telemetry.EventDriveThroughServed:  "drive through scontato",
	telemetry.EventStopGoServed:        "stop and go scontato",
	telemetry.EventFlashback:           "flashback",
	telemetry.EventButtons:             "tasti premuti",
	telemetry.EventRedFlag:             "bandiera rossa",
	telemetry.EventOvertake:            "sorpasso",
	telemetry.EventSafetyCar:           "safety car",
	telemetry.EventCollision:           "collisione",
	telemetry.EventPartialModeEnabled:  "modalità parziale abilitata",
	telemetry.EventPartialModeDisabled: "modalità parziale disabilitata",
	telemetry.EventOvertakeEnabled:     "overtake mode abilitato",
	telemetry.EventOvertakeDisabled:    "overtake mode disabilitato",
}

// Eventi da segnalare a schermo durante la registrazione.
var notableEvents = []string{
	telemetry.EventSessionStarted, telemetry.EventSessionEnded, telemetry.EventChequeredFlag,
	telemetry.EventRaceWinner, telemetry.EventFastestLap, telemetry.EventSafetyCar, telemetry.EventRedFlag,
}

func main() {
	addr := flag.String("addr", ":20777", "indirizzo UDP su cui ascoltare")
	out := flag.String("out", "", "file di output (default recordings/<data-ora>.f1rec.gz)")
	info := flag.String("info", "", "riassume il contenuto della registrazione indicata ed esce")
	replayFile := flag.String("replay", "", "rimanda la registrazione indicata via UDP, come se fosse il gioco in diretta")
	replayTo := flag.String("to", "127.0.0.1:20777", "destinazione di -replay")
	speed := flag.Float64("speed", 1, "velocità di -replay (2 = doppia velocità)")
	flag.Parse()

	if *replayFile != "" {
		if err := replay(*replayFile, *replayTo, *speed); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *info != "" {
		if err := summarize(*info); err != nil {
			log.Fatal(err)
		}
		return
	}

	path := *out
	if path == "" {
		path = filepath.Join("recordings", time.Now().Format("2006-01-02_15-04-05")+".f1rec.gz")
	}
	if err := record(*addr, path); err != nil {
		log.Fatal(err)
	}
}

func record(addr, path string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("impossibile ascoltare su %s (un altro programma usa la porta?): %w", addr, err)
	}
	defer conn.Close()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	w, err := recording.NewWriter(file)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("In ascolto su %s, registro in %s\n", addr, path)
	fmt.Println("Entra in pista nel gioco; premi Ctrl+C per terminare.")

	var (
		start      time.Time // arrivo del primo pacchetto: gli istanti sono relativi a questo
		packets    int
		totalBytes int64
		lastFlush  = time.Now()
		lastStatus = time.Now()
		sessionUID uint64
		buf        = make([]byte, 4096)
	)

	for ctx.Err() == nil {
		// Timeout breve per controllare Ctrl+C e scaricare su file anche quando non arriva nulla.
		conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, _, err := conn.ReadFromUDP(buf)
		now := time.Now()

		if err == nil {
			if start.IsZero() {
				start = now
				fmt.Println("Primo pacchetto ricevuto, registrazione in corso.")
			}
			if err := w.Write(now.Sub(start), buf[:n]); err != nil {
				return err
			}
			packets++
			totalBytes += int64(n)
			sessionUID = announce(buf[:n], now.Sub(start), sessionUID)
		} else if !errors.Is(err, os.ErrDeadlineExceeded) {
			return err
		}

		if now.Sub(lastFlush) >= time.Second {
			if err := w.Flush(); err != nil {
				return err
			}
			lastFlush = now
		}
		if now.Sub(lastStatus) >= 5*time.Second && packets > 0 {
			fmt.Printf("[%s] %d pacchetti, %.1f MB\n", formatDuration(now.Sub(start)), packets, float64(totalBytes)/1e6)
			lastStatus = now
		}
	}

	if err := w.Close(); err != nil {
		return err
	}
	if packets == 0 {
		file.Close()
		os.Remove(path)
		fmt.Println("\nNessun pacchetto ricevuto: file non creato. Controlla le impostazioni di telemetria del gioco.")
		return nil
	}
	fmt.Printf("\nRegistrazione salvata: %s (%d pacchetti)\n", path, packets)
	return nil
}

// announce stampa i momenti importanti e restituisce la sessione corrente.
func announce(packet []byte, at time.Duration, sessionUID uint64) uint64 {
	h, err := telemetry.ParseHeader(packet)
	if err != nil {
		return sessionUID
	}
	stamp := formatDuration(at)
	if h.SessionUID != 0 && h.SessionUID != sessionUID {
		fmt.Printf("[%s] nuova sessione %016x (formato %s, gioco %d.%02d)\n",
			stamp, h.SessionUID, formatLabel(h.PacketFormat), h.GameMajorVersion, h.GameMinorVersion)
		if !telemetry.SupportedFormat(h.PacketFormat) {
			fmt.Printf("[%s] attenzione: il formato %d viene registrato ma non potrà essere decodificato; "+
				"nel gioco imposta Formato UDP su 2025 o 2026\n", stamp, h.PacketFormat)
		}
		sessionUID = h.SessionUID
	}
	if code, ok := telemetry.EventCode(packet); ok && slices.Contains(notableEvents, code) {
		fmt.Printf("[%s] evento %s: %s\n", stamp, code, eventNames[code])
	}
	if h.PacketID == telemetry.PacketFinalClassification {
		fmt.Printf("[%s] classifica finale ricevuta: puoi terminare con Ctrl+C\n", stamp)
	}
	return sessionUID
}

// sessionSummary raccoglie, per una sessione, gli ultimi pacchetti utili al riepilogo.
type sessionSummary struct {
	uid            uint64
	start, end     time.Duration
	session        *telemetry.Session
	participants   *telemetry.Participants
	classification *telemetry.FinalClassification
}

func summarize(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	r, err := recording.NewReader(file)
	if err != nil {
		return err
	}

	var (
		total     int
		size      int64
		last      time.Duration
		counts    = map[telemetry.PacketID]int{}
		formats   = map[uint16]int{}
		events    = map[string]int{}
		sessions  []*sessionSummary
		byUID     = map[uint64]*sessionSummary{}
		parseErrs int
		readErr   error
	)
	for {
		rec, err := r.Next()
		if err != nil {
			if err != io.EOF {
				readErr = err
			}
			break
		}
		total++
		size += int64(len(rec.Data))
		last = rec.At

		p, err := telemetry.Parse(rec.Data)
		if err != nil {
			parseErrs++
			if h, err := telemetry.ParseHeader(rec.Data); err == nil {
				counts[h.PacketID]++
				formats[h.PacketFormat]++
			}
			continue
		}
		h := p.PacketHeader()
		counts[h.PacketID]++
		formats[h.PacketFormat]++
		if e, ok := p.(*telemetry.Event); ok {
			events[e.Code]++
		}
		if h.SessionUID == 0 {
			continue
		}
		s := byUID[h.SessionUID]
		if s == nil {
			s = &sessionSummary{uid: h.SessionUID, start: rec.At}
			byUID[h.SessionUID] = s
			sessions = append(sessions, s)
		}
		s.end = rec.At

		switch p := p.(type) {
		case *telemetry.Session:
			s.session = p
		case *telemetry.Participants:
			s.participants = p
		case *telemetry.FinalClassification:
			s.classification = p
		}
	}

	fmt.Printf("%s\n  durata %s, %d pacchetti, %.1f MB non compressi\n", path, formatDuration(last), total, float64(size)/1e6)
	if readErr != nil {
		fmt.Printf("  attenzione: file troncato (%v), letti i pacchetti fino a quel punto\n", readErr)
	}
	if parseErrs > 0 {
		fmt.Printf("  attenzione: %d pacchetti non decodificabili\n", parseErrs)
	}
	for format, n := range formats {
		fmt.Printf("  formato %s: %d pacchetti\n", formatLabel(format), n)
	}

	for i, s := range sessions {
		printSession(i+1, s)
	}

	fmt.Println("\npacchetti per tipo:")
	ids := make([]telemetry.PacketID, 0, len(counts))
	for id := range counts {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		fmt.Printf("  %-20s %7d\n", id, counts[id])
	}
	if len(events) > 0 {
		fmt.Println("eventi:")
		codes := make([]string, 0, len(events))
		for code := range events {
			codes = append(codes, code)
		}
		slices.Sort(codes)
		for _, code := range codes {
			fmt.Printf("  %s %5d  %s\n", code, events[code], eventNames[code])
		}
	}
	return nil
}

func printSession(n int, s *sessionSummary) {
	title := "tipo sconosciuto"
	if s.session != nil {
		title = fmt.Sprintf("%s – %s", s.session.TrackID, s.session.SessionType)
		if s.session.SessionType.IsRace() {
			title += fmt.Sprintf(", %d giri", s.session.TotalLaps)
		}
	}
	fmt.Printf("\nSessione %d: %s (da %s a %s, id %016x)\n", n, title, formatDuration(s.start), formatDuration(s.end), s.uid)

	if s.participants != nil {
		fmt.Printf("  %d piloti\n", s.participants.NumActiveCars)
	}
	if s.classification == nil {
		fmt.Println("  nessuna classifica finale")
		return
	}

	// La classifica è indicizzata per auto: la riordino per posizione.
	type row struct {
		idx   int
		entry telemetry.ClassificationEntry
	}
	var rows []row
	for i, c := range s.classification.Cars {
		if c.Position > 0 {
			rows = append(rows, row{i, c})
		}
	}
	slices.SortFunc(rows, func(a, b row) int { return int(a.entry.Position) - int(b.entry.Position) })

	fmt.Println("  classifica finale:")
	for _, r := range rows {
		name, team := fmt.Sprintf("auto %d", r.idx), ""
		if s.participants != nil && r.idx < len(s.participants.Cars) {
			p := s.participants.Cars[r.idx]
			name, team = p.Name, p.TeamID.String()
		}
		result := formatRaceTime(r.entry.TotalRaceTime)
		if r.entry.ResultStatus != telemetry.ResultFinished {
			result = r.entry.ResultStatus.String()
		}
		player := ""
		if h := s.classification.Header; r.idx == int(h.PlayerCarIndex) {
			player = "  ◀ giocatore"
		} else if r.idx == int(h.SecondaryPlayerCarIndex) {
			player = "  ◀ giocatore 2"
		}
		fmt.Printf("  %3d. %-20s %-16s %2d giri  %-12s  miglior giro %s  %2d punti%s\n",
			r.entry.Position, name, team, r.entry.NumLaps, result, formatLapTime(r.entry.BestLapTimeMS), r.entry.Points, player)
	}
}

// formatLapTime formatta un tempo sul giro in m:ss.mmm.
func formatLapTime(ms uint32) string {
	if ms == 0 {
		return "   -    "
	}
	return fmt.Sprintf("%d:%02d.%03d", ms/60_000, ms/1000%60, ms%1000)
}

// formatRaceTime formatta un tempo totale di gara (in secondi) in h:mm:ss.mmm o m:ss.mmm.
func formatRaceTime(seconds float64) string {
	ms := uint64(seconds*1000 + 0.5)
	if ms >= 3_600_000 {
		return fmt.Sprintf("%d:%02d:%02d.%03d", ms/3_600_000, ms/60_000%60, ms/1000%60, ms%1000)
	}
	return fmt.Sprintf("%d:%02d.%03d", ms/60_000, ms/1000%60, ms%1000)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	return fmt.Sprintf("%02d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}

func formatLabel(format uint16) string {
	switch format {
	case telemetry.Format2025:
		return "2025, F1 25"
	case telemetry.Format2026:
		return "2026, F1 25 + 2026 Season Pack"
	}
	return fmt.Sprintf("%d, non supportato", format)
}

// replay invia i pacchetti di una registrazione a to, rispettando i tempi originali
// (divisi per speed). Serve a provare il server e la pagina live senza il gioco.
func replay(path, to string, speed float64) error {
	if speed <= 0 {
		return fmt.Errorf("velocità non valida: %v", speed)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	r, err := recording.NewReader(file)
	if err != nil {
		return err
	}
	conn, err := net.Dial("udp", to)
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("Invio %s a %s (velocità %gx); Ctrl+C per interrompere.\n", path, to, speed)
	start := time.Now()
	var sessionUID uint64
	sent := 0
	for {
		p, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// Aspetta l'istante in cui il pacchetto era arrivato durante la registrazione.
		if wait := time.Until(start.Add(time.Duration(float64(p.At) / speed))); wait > 0 {
			select {
			case <-ctx.Done():
				fmt.Printf("\nInterrotto dopo %d pacchetti.\n", sent)
				return nil
			case <-time.After(wait):
			}
		}
		if _, err := conn.Write(p.Data); err != nil {
			return err
		}
		sent++
		sessionUID = announce(p.Data, p.At, sessionUID)
	}
	fmt.Printf("\nInviati %d pacchetti in %s.\n", sent, time.Since(start).Round(time.Second))
	return nil
}
