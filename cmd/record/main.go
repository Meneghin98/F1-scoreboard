// Comando record: registra su file i pacchetti UDP della telemetria di F1 25.
//
//	go run ./cmd/record                  registra in recordings/<data-ora>.f1rec.gz finché non premi Ctrl+C
//	go run ./cmd/record -out gara.f1rec.gz
//	go run ./cmd/record -info gara.f1rec.gz   riassume il contenuto di una registrazione
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

// Eventi da segnalare a schermo durante la registrazione.
var notableEvents = map[string]string{
	"SSTA": "inizio sessione",
	"SEND": "fine sessione",
	"CHQF": "bandiera a scacchi",
	"RCWN": "vincitore della gara",
	"FTLP": "giro più veloce",
	"SCAR": "safety car",
	"RDFL": "bandiera rossa",
}

func main() {
	addr := flag.String("addr", ":20777", "indirizzo UDP su cui ascoltare")
	out := flag.String("out", "", "file di output (default recordings/<data-ora>.f1rec.gz)")
	info := flag.String("info", "", "riassume il contenuto della registrazione indicata ed esce")
	flag.Parse()

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
	if code, ok := telemetry.EventCode(packet); ok {
		if desc, notable := notableEvents[code]; notable {
			fmt.Printf("[%s] evento %s: %s\n", stamp, code, desc)
		}
	}
	if h.PacketID == telemetry.PacketFinalClassification {
		fmt.Printf("[%s] classifica finale ricevuta: puoi terminare con Ctrl+C\n", stamp)
	}
	return sessionUID
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
		total    int
		size     int64
		last     time.Duration
		counts   = map[telemetry.PacketID]int{}
		formats  = map[uint16]int{}
		events   = map[string]int{}
		sessions []uint64
		readErr  error
	)
	for {
		p, err := r.Next()
		if err != nil {
			if err != io.EOF {
				readErr = err
			}
			break
		}
		total++
		size += int64(len(p.Data))
		last = p.At
		h, err := telemetry.ParseHeader(p.Data)
		if err != nil {
			continue
		}
		counts[h.PacketID]++
		formats[h.PacketFormat]++
		if h.SessionUID != 0 && !slices.Contains(sessions, h.SessionUID) {
			sessions = append(sessions, h.SessionUID)
		}
		if code, ok := telemetry.EventCode(p.Data); ok {
			events[code]++
		}
	}

	fmt.Printf("%s\n  durata %s, %d pacchetti, %.1f MB non compressi\n", path, formatDuration(last), total, float64(size)/1e6)
	if readErr != nil {
		fmt.Printf("  attenzione: file troncato (%v), letti i pacchetti fino a quel punto\n", readErr)
	}
	for format, n := range formats {
		fmt.Printf("  formato %s: %d pacchetti\n", formatLabel(format), n)
	}
	fmt.Printf("  sessioni: %d\n", len(sessions))
	for _, uid := range sessions {
		fmt.Printf("    %016x\n", uid)
	}
	fmt.Println("  pacchetti per tipo:")
	ids := make([]telemetry.PacketID, 0, len(counts))
	for id := range counts {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		fmt.Printf("    %-20s %7d\n", id, counts[id])
	}
	if len(events) > 0 {
		fmt.Println("  eventi:")
		codes := make([]string, 0, len(events))
		for code := range events {
			codes = append(codes, code)
		}
		slices.Sort(codes)
		for _, code := range codes {
			fmt.Printf("    %s %5d  %s\n", code, events[code], notableEvents[code])
		}
	}
	return nil
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
