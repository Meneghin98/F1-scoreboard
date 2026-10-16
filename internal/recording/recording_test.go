package recording

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	packets := []Packet{
		{At: 0, Data: []byte("primo")},
		{At: 50 * time.Millisecond, Data: bytes.Repeat([]byte{0xAB}, 1460)},
		{At: 2 * time.Second, Data: []byte{}},
	}

	var file bytes.Buffer
	w, err := NewWriter(&file)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range packets {
		if err := w.Write(p.At, p.Data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := NewReader(&file)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range packets {
		got, err := r.Next()
		if err != nil {
			t.Fatalf("pacchetto %d: %v", i, err)
		}
		if got.At != want.At || !bytes.Equal(got.Data, want.Data) {
			t.Fatalf("pacchetto %d: %v/%d byte, atteso %v/%d byte", i, got.At, len(got.Data), want.At, len(want.Data))
		}
	}
	if _, err := r.Next(); err != io.EOF {
		t.Fatalf("a fine file: %v, atteso io.EOF", err)
	}
}

// Una registrazione interrotta senza Close resta leggibile fino all'ultimo Flush.
func TestFlushWithoutClose(t *testing.T) {
	var file bytes.Buffer
	w, _ := NewWriter(&file)
	w.Write(0, []byte("salvato"))
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	w.Write(time.Second, []byte("perso")) // non ancora scaricato su file

	r, err := NewReader(bytes.NewReader(file.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	p, err := r.Next()
	if err != nil || string(p.Data) != "salvato" {
		t.Fatalf("primo pacchetto: %q, %v", p.Data, err)
	}
	if _, err := r.Next(); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("dopo l'ultimo flush: %v, atteso io.ErrUnexpectedEOF", err)
	}
}

func TestNotARecording(t *testing.T) {
	if _, err := NewReader(bytes.NewReader([]byte("ciao"))); err == nil {
		t.Fatal("un file qualsiasi non deve essere accettato")
	}
}
