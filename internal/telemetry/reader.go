package telemetry

import (
	"bytes"
	"encoding/binary"
	"math"
)

// reader legge in sequenza i campi little-endian di un pacchetto.
// Se il pacchetto finisce prima del previsto non va in panic: restituisce
// valori zero e segna short, che Parse trasforma in errore.
type reader struct {
	buf   []byte
	off   int
	short bool
	wide  bool // formato 2026: alcuni ID passano da uint8 a uint16
}

func (r *reader) take(n int) []byte {
	if r.off+n > len(r.buf) {
		r.short = true
		r.off = len(r.buf)
		return make([]byte, n)
	}
	b := r.buf[r.off : r.off+n]
	r.off += n
	return b
}

func (r *reader) skip(n int)         { r.take(n) }
func (r *reader) u8() uint8          { return r.take(1)[0] }
func (r *reader) i8() int8           { return int8(r.u8()) }
func (r *reader) flag() bool         { return r.u8() != 0 }
func (r *reader) u16() uint16        { return binary.LittleEndian.Uint16(r.take(2)) }
func (r *reader) i16() int16         { return int16(r.u16()) }
func (r *reader) u32() uint32        { return binary.LittleEndian.Uint32(r.take(4)) }
func (r *reader) f32() float32       { return math.Float32frombits(r.u32()) }
func (r *reader) f64() float64       { return math.Float64frombits(binary.LittleEndian.Uint64(r.take(8))) }
func (r *reader) vec3() Vector3      { return Vector3{r.f32(), r.f32(), r.f32()} }
func (r *reader) wheels() [4]float32 { return [4]float32{r.f32(), r.f32(), r.f32(), r.f32()} }

func (r *reader) wheelsU8() [4]uint8 {
	var w [4]uint8
	copy(w[:], r.take(4))
	return w
}

// id legge un identificativo di pilota, rete o team: uint8 nel 2025, uint16 nel 2026.
func (r *reader) id() uint16 {
	if r.wide {
		return r.u16()
	}
	return uint16(r.u8())
}

// str legge una stringa C a lunghezza fissa (terminata da zero).
func (r *reader) str(n int) string {
	b := r.take(n)
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}

// minutesMS combina le parti "millisecondi" e "minuti" con cui il gioco invia alcuni tempi.
func (r *reader) minutesMS() uint32 {
	ms := uint32(r.u16())
	return ms + uint32(r.u8())*60_000
}

// count limita un contatore letto dal pacchetto alla capacità dell'array che descrive.
func count(n uint8, capacity int) int {
	return min(int(n), capacity)
}
