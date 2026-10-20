// F1 Scoreboard: segnapunti web per i tornei tra amici su F1 25.
package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	// Fusi orari dentro l'eseguibile: l'ora locale (date delle gare, nomi dei
	// file) funziona con TZ=Europe/Rome anche su un sistema senza i dati dei fusi.
	_ "time/tzdata"

	"f1-scoreboard/internal/api"
	"f1-scoreboard/internal/backup"
	"f1-scoreboard/internal/live"
	"f1-scoreboard/internal/store"
	"f1-scoreboard/web"
)

func main() {
	port := envOr("PORT", "3000")
	dbPath := envOr("DB_PATH", filepath.Join("data", "scoreboard.db"))
	telemetryAddr := envOr("TELEMETRY_ADDR", ":20777")
	sessionsDir := envOr("TELEMETRY_DIR", filepath.Join(filepath.Dir(dbPath), "sessions"))
	record := !strings.EqualFold(os.Getenv("TELEMETRY_RECORD"), "false")
	backupDir := envOr("BACKUP_DIR", filepath.Join(filepath.Dir(dbPath), "backups"))
	backupKeep, err := strconv.Atoi(envOr("BACKUP_KEEP", "7"))
	if err != nil || backupKeep < 0 {
		log.Fatalf("BACKUP_KEEP deve essere un numero intero da 0 in su, non %q", os.Getenv("BACKUP_KEEP"))
	}

	db, err := store.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Di default il frontend è quello incorporato nel binario; con WEB_DIR
	// lo si legge dal disco, utile in sviluppo per vedere subito le modifiche.
	var static fs.FS = web.Files()
	if dir := os.Getenv("WEB_DIR"); dir != "" {
		static = os.DirFS(dir)
	}
	if _, err := fs.Stat(static, "index.html"); err != nil {
		log.Print("attenzione: frontend non trovato, esegui \"npm run build\" in web/ prima di go build")
	}

	// Ctrl+C o SIGTERM (es. systemctl stop) annullano ctx e avviano lo spegnimento.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// La telemetria è sempre in ascolto: appena il gioco invia dati la sessione
	// viene rilevata e registrata, senza configurazioni.
	liveService := live.NewService(db, live.Options{Dir: sessionsDir, Record: record, PrepareReplays: true})
	go liveService.Run(ctx, telemetryAddr)

	// Un backup del database al giorno, tenendo gli ultimi backupKeep (0 = nessuno).
	backups := backup.New(db, dbPath, backupDir, backupKeep)
	go backups.Run(ctx)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: api.New(db, static, liveService, backups),
		// Un client che non manda nemmeno le intestazioni non tiene occupata la
		// connessione per sempre. Niente WriteTimeout: lo stream live dura ore.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		// Prima si chiude la telemetria (salva la registrazione in corso e chiude
		// gli stream live) e si interrompe un eventuale backup, poi il server
		// HTTP lascia finire le richieste in corso.
		<-liveService.Done()
		<-backups.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Printf("F1 Scoreboard in ascolto su http://localhost:%s", port)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	// ListenAndServe ritorna appena inizia lo spegnimento: aspettiamo che
	// Shutdown abbia finito prima di chiudere il database.
	<-shutdownDone
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
