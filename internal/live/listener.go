// Package live riceve la telemetria del gioco, ne ricava lo stato della sessione
// in corso e registra ogni sessione su file.
package live

import (
	"context"
	"errors"
	"log"
	"net"
	"time"
)

// retryDelay è l'attesa prima di riprovare ad aprire la porta UDP.
const retryDelay = 10 * time.Second

// Listener riceve i pacchetti UDP della telemetria.
type Listener struct {
	Addr   string                          // es. ":20777"
	Handle func(at time.Time, data []byte) // data è valido solo durante la chiamata
	Ready  func(addr net.Addr)             // opzionale, chiamata quando la porta è aperta
}

// Run ascolta finché ctx non viene annullato. Se la porta non si può aprire
// (per esempio perché la usa il registratore) lo segnala e riprova, senza
// fermare il resto del server.
func (l *Listener) Run(ctx context.Context) {
	for {
		err := l.listen(ctx)
		if ctx.Err() != nil {
			return
		}
		log.Printf("telemetria: %v, nuovo tentativo tra %s", err, retryDelay)
		select {
		case <-ctx.Done():
			return
		case <-time.After(retryDelay):
		}
	}
}

func (l *Listener) listen(ctx context.Context) error {
	conn, err := net.ListenPacket("udp", l.Addr)
	if err != nil {
		return err
	}
	log.Printf("telemetria: in ascolto su UDP %s", conn.LocalAddr())
	if l.Ready != nil {
		l.Ready(conn.LocalAddr())
	}

	// Chiudere la connessione sblocca ReadFrom quando il server si spegne.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
		case <-stop:
		}
		conn.Close()
	}()

	buf := make([]byte, 4096) // i pacchetti F1 non superano i 1500 byte
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		l.Handle(time.Now(), buf[:n])
	}
}
