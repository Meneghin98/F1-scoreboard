// Package recording salva e rilegge i pacchetti UDP della telemetria, così si
// possono sviluppare e testare parser e pagine live senza il gioco acceso.
//
// Formato del file (tutto compresso con gzip, interi little-endian):
//
//	"F1REC" + versione (1 byte)
//	ripetuto per ogni pacchetto:
//	  istante di arrivo in nanosecondi dall'inizio della registrazione (uint64)
//	  lunghezza del pacchetto (uint32)
//	  pacchetto grezzo, così come è arrivato dal gioco
package recording

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	magic   = "F1REC"
	version = 1

	// Nessun pacchetto F1 supera i 2 KB: un valore più grande indica un file corrotto.
	maxPacketSize = 64 * 1024
)

// Packet è un pacchetto registrato con il suo istante di arrivo.
type Packet struct {
	At   time.Duration // tempo trascorso dall'inizio della registrazione
	Data []byte
}

// Writer scrive una registrazione.
type Writer struct {
	gz *gzip.Writer
	bw *bufio.Writer
}

// NewWriter scrive l'intestazione su w. Va chiuso con Close, che però non chiude w.
func NewWriter(w io.Writer) (*Writer, error) {
	gz := gzip.NewWriter(w)
	bw := bufio.NewWriter(gz)
	if _, err := bw.WriteString(magic); err != nil {
		return nil, err
	}
	if err := bw.WriteByte(version); err != nil {
		return nil, err
	}
	return &Writer{gz: gz, bw: bw}, nil
}

// Write aggiunge un pacchetto arrivato all'istante at.
func (w *Writer) Write(at time.Duration, packet []byte) error {
	var head [12]byte
	binary.LittleEndian.PutUint64(head[0:8], uint64(at))
	binary.LittleEndian.PutUint32(head[8:12], uint32(len(packet)))
	if _, err := w.bw.Write(head[:]); err != nil {
		return err
	}
	_, err := w.bw.Write(packet)
	return err
}

// Flush rende leggibile tutto quello scritto finora: se il programma viene
// interrotto senza Close, la registrazione resta valida fino all'ultimo Flush.
func (w *Writer) Flush() error {
	if err := w.bw.Flush(); err != nil {
		return err
	}
	return w.gz.Flush()
}

// Close completa il file.
func (w *Writer) Close() error {
	if err := w.bw.Flush(); err != nil {
		return err
	}
	return w.gz.Close()
}

// Reader legge una registrazione.
type Reader struct {
	r *bufio.Reader
}

// NewReader verifica l'intestazione della registrazione letta da r.
func NewReader(r io.Reader) (*Reader, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("non è una registrazione valida: %w", err)
	}
	br := bufio.NewReader(gz)
	head := make([]byte, len(magic)+1)
	if _, err := io.ReadFull(br, head); err != nil || string(head[:len(magic)]) != magic {
		return nil, errors.New("non è una registrazione valida: intestazione mancante")
	}
	if head[len(magic)] != version {
		return nil, fmt.Errorf("versione di registrazione %d non supportata", head[len(magic)])
	}
	return &Reader{r: br}, nil
}

// Next restituisce il pacchetto successivo, oppure io.EOF a fine file.
// Un file troncato (registrazione interrotta) termina con io.ErrUnexpectedEOF.
func (r *Reader) Next() (Packet, error) {
	var head [12]byte
	if _, err := io.ReadFull(r.r, head[:]); err != nil {
		if errors.Is(err, io.EOF) {
			return Packet{}, io.EOF
		}
		return Packet{}, io.ErrUnexpectedEOF
	}
	size := binary.LittleEndian.Uint32(head[8:12])
	if size > maxPacketSize {
		return Packet{}, fmt.Errorf("pacchetto di %d byte: file corrotto", size)
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(r.r, data); err != nil {
		return Packet{}, io.ErrUnexpectedEOF
	}
	return Packet{At: time.Duration(binary.LittleEndian.Uint64(head[0:8])), Data: data}, nil
}
