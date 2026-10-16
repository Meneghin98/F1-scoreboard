package telemetry

import (
	"bytes"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"f1-scoreboard/internal/recording"
)

// TestRecordings decodifica tutte le registrazioni reali in testdata/recordings:
// ogni pacchetto deve consumare esattamente i suoi byte e contenere valori plausibili
// (un campo fuori posto produrrebbe velocità assurde, posizioni duplicate, ecc.).
func TestRecordings(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "recordings", "*.f1rec.gz"))
	if len(files) == 0 {
		t.Skip("nessuna registrazione in testdata/recordings")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) { checkRecording(t, file) })
	}
}

func checkRecording(t *testing.T, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.HasPrefix(data, []byte("version https://git-lfs")) {
		t.Skip("registrazione non scaricata da Git LFS: esegui git lfs pull")
	}
	r, err := recording.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	counts := map[PacketID]int{}
	active := map[int]bool{} // auto in pista secondo l'ultimo LapData
	failures := 0
	fail := func(format string, args ...any) {
		failures++
		if failures <= 20 {
			t.Errorf(format, args...)
		}
	}

	for {
		rec, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		p, err := parse(rec.Data, true)
		if err != nil {
			fail("pacchetto a %v: %v", rec.At, err)
			continue
		}
		counts[p.PacketHeader().PacketID]++

		switch p := p.(type) {
		case *LapData:
			clear(active)
			seen := map[uint8]bool{}
			for i, c := range p.Cars {
				if c.ResultStatus < ResultActive {
					continue
				}
				active[i] = true
				if c.CarPosition < 1 || int(c.CarPosition) > len(p.Cars) || seen[c.CarPosition] {
					fail("LapData: posizione %d non valida o duplicata (auto %d)", c.CarPosition, i)
				}
				seen[c.CarPosition] = true
				if c.CurrentLapNum > 100 || c.LastLapTimeMS > 10*60_000 {
					fail("LapData: giro %d, ultimo tempo %d ms (auto %d)", c.CurrentLapNum, c.LastLapTimeMS, i)
				}
			}
		case *CarTelemetry:
			for i := range active {
				c := p.Cars[i]
				if c.Speed > 400 || c.Throttle < 0 || c.Throttle > 1 || c.Brake < 0 || c.Brake > 1 ||
					c.Steer < -1 || c.Steer > 1 || c.Gear < -1 || c.Gear > 8 || c.EngineTemperature > 200 {
					fail("CarTelemetry: valori impossibili per l'auto %d: %+v", i, c)
				}
			}
		case *Motion:
			for i := range active {
				f := p.Cars[i].WorldForwardDir
				if n := math.Sqrt(float64(f.X*f.X + f.Y*f.Y + f.Z*f.Z)); n < 0.95 || n > 1.05 {
					fail("Motion: direzione non normalizzata (%.2f) per l'auto %d", n, i)
				}
			}
		case *CarStatus:
			for i := range active {
				c := p.Cars[i]
				if c.FuelInTank < 0 || c.FuelInTank > 150 || c.MaxGears > 9 || c.TyresAgeLaps > 100 {
					fail("CarStatus: valori impossibili per l'auto %d: %+v", i, c)
				}
			}
		case *Session:
			if _, known := trackNames[p.TrackID]; !known || int(p.SessionType) >= len(sessionNames) ||
				p.TrackLength < 3000 || p.TrackLength > 8000 || p.TotalLaps > 100 {
				fail("Session: circuito %d, tipo %d, lunghezza %d, giri %d", p.TrackID, p.SessionType, p.TrackLength, p.TotalLaps)
			}
		case *Participants:
			if p.NumActiveCars < 1 || int(p.NumActiveCars) > len(p.Cars) {
				fail("Participants: %d auto attive", p.NumActiveCars)
			}
			for i := range active {
				if name := p.Cars[i].Name; name == "" || !utf8.ValidString(name) {
					fail("Participants: nome %q non valido per l'auto %d", name, i)
				}
			}
		case *FinalClassification:
			seen := map[uint8]bool{}
			for _, c := range p.Cars {
				if c.Position == 0 {
					continue
				}
				if int(c.Position) > int(p.NumCars) || seen[c.Position] {
					fail("FinalClassification: posizione %d non valida o duplicata (%d auto)", c.Position, p.NumCars)
				}
				seen[c.Position] = true
			}
			if len(seen) != int(p.NumCars) {
				fail("FinalClassification: %d posizioni per %d auto", len(seen), p.NumCars)
			}
		case *SessionHistory:
			for _, lap := range p.Laps {
				if lap.LapTimeMS > 10*60_000 {
					fail("SessionHistory: giro di %d ms (auto %d)", lap.LapTimeMS, p.CarIdx)
				}
			}
		}
	}

	if failures > 20 {
		t.Errorf("... e altri %d errori", failures-20)
	}
	t.Logf("pacchetti decodificati: %v", counts)
}
