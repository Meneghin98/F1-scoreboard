package live

import (
	"math"

	"f1-scoreboard/internal/telemetry"
)

// Il gioco non invia il disegno del circuito: lo si ricostruisce dalle
// posizioni delle auto. Il giro è diviso in tratti di outlineStep metri e per
// ogni tratto si tiene il primo punto in cui passa un'auto. Con molte auto in
// pista il tracciato è completo in meno di un giro.
const outlineStep = 10 // metri

// Point è una posizione sul piano orizzontale del circuito, in metri.
// Nel sistema del gioco Y è l'altezza: la pianta usa X e Z.
type Point struct {
	X float32 `json:"x"`
	Z float32 `json:"z"`
}

// TrackMap è la pianta del circuito ricostruita finora.
type TrackMap struct {
	Length int     `json:"length"` // metri
	Points []Point `json:"points"` // in ordine di percorrenza; i tratti non ancora visti mancano
}

type outline struct {
	length int
	points []Point
	seen   []bool
	filled int
}

func newOutline(length int) *outline {
	n := (length + outlineStep - 1) / outlineStep
	return &outline{length: length, points: make([]Point, n), seen: make([]bool, n)}
}

// add registra le posizioni delle auto in pista. Le auto ai box sono escluse:
// la corsia dei box non fa parte del tracciato.
func (o *outline) add(laps *telemetry.LapData, motion *telemetry.Motion) {
	for i, lap := range laps.Cars {
		if i >= len(motion.Cars) || lap.ResultStatus != telemetry.ResultActive || lap.PitStatus != 0 {
			continue
		}
		// Prima del primo passaggio sul traguardo la distanza è negativa.
		d := lap.LapDistance
		if d < 0 || int(d) >= o.length {
			continue
		}
		b := int(d) / outlineStep
		if o.seen[b] {
			continue
		}
		pos := motion.Cars[i].WorldPosition
		// Le auto non ancora in gioco hanno posizione (0, 0, 0).
		if pos.X == 0 && pos.Z == 0 {
			continue
		}
		o.points[b] = Point{X: round1(pos.X), Z: round1(pos.Z)}
		o.seen[b] = true
		o.filled++
	}
}

func (o *outline) trackMap() TrackMap {
	m := TrackMap{Length: o.length, Points: make([]Point, 0, o.filled)}
	for i, p := range o.points {
		if o.seen[i] {
			m.Points = append(m.Points, p)
		}
	}
	return m
}

// round1 arrotonda ai 10 cm: più che sufficienti per una mappa e il JSON resta corto.
func round1(v float32) float32 {
	return float32(math.Round(float64(v)*10) / 10)
}
