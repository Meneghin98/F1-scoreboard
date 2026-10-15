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
	"syscall"
	"time"

	"f1-scoreboard/internal/api"
	"f1-scoreboard/internal/store"
	"f1-scoreboard/web"
)

func main() {
	port := envOr("PORT", "3000")
	dbPath := envOr("DB_PATH", filepath.Join("data", "scoreboard.db"))

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

	srv := &http.Server{Addr: ":" + port, Handler: api.New(db, static)}

	// Ctrl+C o SIGTERM (es. systemctl stop): chiude il server lasciando
	// finire le richieste in corso, poi il defer chiude il database.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
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
