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
	"f1-scoreboard/internal/update"
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
	exe, err := executable()
	if err != nil {
		log.Fatal(err)
	}
	updates := !strings.EqualFold(os.Getenv("UPDATE"), "false")
	installHour, err := strconv.Atoi(envOr("UPDATE_HOUR", "4"))
	if err != nil || installHour < 0 || installHour > 23 {
		log.Fatalf("UPDATE_HOUR deve essere un'ora da 0 a 23, non %q", os.Getenv("UPDATE_HOUR"))
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

	// Un backup del database al giorno, tenendo gli ultimi backupKeep (0 = nessuno).
	backups := backup.New(db, dbPath, backupDir, backupKeep)

	// Una versione scaricata e non ancora installata (per esempio perché il
	// server è stato chiuso senza spegnerlo bene) si installa subito, prima di
	// avviare il resto.
	if updates && installUpdate(exe, backups) {
		db.Close()
		restart(exe)
	}

	// Ctrl+C o SIGTERM (es. systemctl stop) annullano ctx e avviano lo
	// spegnimento; lo avvia anche l'aggiornamento, con causa errRestart.
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancelCause(sigCtx)
	defer cancel(nil)

	// La telemetria è sempre in ascolto: appena il gioco invia dati la sessione
	// viene rilevata e registrata, senza configurazioni.
	liveService := live.NewService(db, live.Options{Dir: sessionsDir, Record: record, PrepareReplays: true})
	go liveService.Run(ctx, telemetryAddr)

	go backups.Run(ctx)

	// Le versioni nuove pubblicate su GitHub si scaricano accanto all'eseguibile
	// (UPDATE=false per non farlo) e si installano allo spegnimento, oppure con
	// un riavvio all'ora UPDATE_HOUR se nessuno sta giocando.
	updater := update.New(update.Options{Exe: exe, Enabled: updates, InstallHour: installHour, Busy: liveService.Busy})
	go updater.Run(ctx)
	go func() {
		select {
		case <-updater.RestartRequested():
			log.Print("riavvio per installare l'aggiornamento")
			cancel(errRestart)
		case <-ctx.Done():
		}
	}()

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: api.New(db, static, liveService, backups, updater),
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

	// Allo spegnimento si installa la versione scaricata, se c'è: partirà al
	// prossimo avvio, o subito se il server si sta riavviando per questo.
	if updates {
		installUpdate(exe, backups)
	}
	if errors.Is(context.Cause(ctx), errRestart) {
		// Anche se l'installazione non è riuscita si riparte, con la versione di prima.
		db.Close()
		restart(exe)
	}
}

// errRestart è la causa dello spegnimento quando il server si riavvia per un aggiornamento.
var errRestart = errors.New("riavvio per l'aggiornamento")

// installUpdate installa la versione scaricata, se c'è, dopo un backup del
// database: la versione nuova potrebbe modificarlo con le sue migrazioni, e
// con il backup si può tornare a quella vecchia. Dice se l'ha installata.
func installUpdate(exe string, backups *backup.Manager) bool {
	if update.Staged(exe) == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := backups.Create(ctx, time.Now()); err != nil {
		log.Printf("aggiornamento non installato, backup del database non riuscito: %v", err)
		return false
	}
	number, err := update.Install(exe)
	if err != nil {
		log.Printf("aggiornamento non installato: %v", err)
		return false
	}
	if number == "" {
		return false
	}
	log.Printf("aggiornamento: installata la versione %s", number)
	return true
}

// restart riavvia il server con l'eseguibile appena installato. Il database
// deve essere già chiuso: i defer non vengono eseguiti.
func restart(exe string) {
	if err := update.Restart(exe); err != nil {
		log.Fatalf("riavvio non riuscito, avvia di nuovo il server a mano: %v", err)
	}
}

// executable restituisce il percorso vero dell'eseguibile in esecuzione: se è
// stato avviato da un link simbolico, quello del file a cui punta.
func executable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
