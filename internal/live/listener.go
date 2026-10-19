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

const (
	// retryDelay è l'attesa prima di riprovare ad aprire la porta UDP.
	retryDelay = 10 * time.Second
	// queueSize è quanti pacchetti possono aspettare di essere elaborati. A 20 Hz
	// il gioco ne manda circa 200 al secondo: la coda assorbe una decina di
	// secondi di rallentamento (per esempio una scrittura lenta sul database)
	// senza perdere pacchetti come la classifica finale.
	queueSize = 2048
	// readBuffer è il buffer di ricezione chiesto al sistema operativo: tiene i
	// pacchetti arrivati mentre il programma non li sta leggendo.
	readBuffer = 4 << 20
)

// Listener riceve i pacchetti UDP della telemetria.
type Listener struct {
	Addr   string                          // es. ":20777"
	Handle func(at time.Time, data []byte) // chiamata da una sola goroutine, nell'ordine di arrivo
	Ready  func(addr net.Addr)             // opzionale, chiamata quando la porta è aperta
	Failed func(err error)                 // opzionale, chiamata quando la porta non si può aprire
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
		if l.Failed != nil {
			l.Failed(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(retryDelay):
		}
	}
}

// packet è un pacchetto ricevuto in attesa di essere elaborato.
type packet struct {
	at   time.Time
	data []byte
}

func (l *Listener) listen(ctx context.Context) error {
	conn, err := net.ListenPacket("udp", l.Addr)
	if err != nil {
		return err
	}
	// Se il sistema concede meno (su Linux il massimo è net.core.rmem_max) si
	// tiene quello che dà: non è un errore.
	if udp, ok := conn.(*net.UDPConn); ok {
		if err := udp.SetReadBuffer(readBuffer); err != nil {
			log.Printf("telemetria: buffer di ricezione UDP: %v", err)
		}
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

	// La lettura e l'elaborazione vanno in due goroutine separate: se
	// l'elaborazione rallenta, i pacchetti aspettano nella coda invece di
	// riempire il buffer del sistema, che oltre un certo limite li scarta.
	queue := make(chan packet, queueSize)
	worked := make(chan struct{})
	go func() {
		defer close(worked)
		for p := range queue {
			l.Handle(p.at, p.data)
		}
	}()
	defer func() {
		close(queue)
		<-worked // i pacchetti già in coda vengono elaborati prima di uscire
	}()

	var (
		dropped    int
		lastReport time.Time
	)
	buf := make([]byte, 4096) // i pacchetti F1 non superano i 1500 byte
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		// buf viene riusato dalla lettura successiva: in coda va una copia.
		p := packet{at: time.Now(), data: append([]byte(nil), buf[:n]...)}
		select {
		case queue <- p:
		default:
			// Coda piena: l'elaborazione è ferma da secondi. Meglio perdere
			// questo pacchetto che bloccare la lettura di tutti gli altri.
			dropped++
			if time.Since(lastReport) > 10*time.Second {
				log.Printf("telemetria: elaborazione in ritardo, %d pacchetti scartati finora", dropped)
				lastReport = time.Now()
			}
		}
	}
}
