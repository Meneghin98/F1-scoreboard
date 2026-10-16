package telemetry

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"f1-scoreboard/internal/recording"
)

// samplePackets restituisce un pacchetto reale per ogni tipo presente nella prima registrazione.
func samplePackets(b *testing.B) map[PacketID][]byte {
	b.Helper()
	files, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "recordings", "*.f1rec.gz"))
	if len(files) == 0 {
		b.Skip("nessuna registrazione in testdata/recordings")
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		b.Fatal(err)
	}
	if bytes.HasPrefix(data, []byte("version https://git-lfs")) {
		b.Skip("registrazione non scaricata da Git LFS")
	}
	r, err := recording.NewReader(bytes.NewReader(data))
	if err != nil {
		b.Fatal(err)
	}
	samples := map[PacketID][]byte{}
	for {
		p, err := r.Next()
		if errors.Is(err, io.EOF) {
			return samples
		}
		if err != nil {
			b.Fatal(err)
		}
		if h, err := ParseHeader(p.Data); err == nil {
			samples[h.PacketID] = p.Data // tiene l'ultimo: a fine sessione i dati sono completi
		}
	}
}

// BenchmarkParse misura la decodifica di ogni tipo di pacchetto.
//
//	go test -bench Parse -benchmem ./internal/telemetry
func BenchmarkParse(b *testing.B) {
	samples := samplePackets(b)
	for id := PacketMotion; id <= PacketCarTelemetry2; id++ {
		data, ok := samples[id]
		if !ok {
			continue
		}
		b.Run(id.String(), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Parse(data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
