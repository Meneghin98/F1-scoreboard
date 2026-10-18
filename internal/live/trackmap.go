package live

import (
	"database/sql"
	"encoding/json"
	"errors"
	"math"

	"f1-scoreboard/internal/telemetry"
)

// Il gioco non invia il disegno del circuito: lo si ricostruisce dalle
// posizioni delle auto. Il giro è diviso in tratti di outlineStep metri e per
// ogni tratto si tiene il primo punto in cui passa un'auto. Con molte auto in
// pista il tracciato è completo in meno di un giro. Una volta completo viene
// salvato nel database (track_maps): le volte successive la mappa c'è subito.
const outlineStep = 10 // metri

// Point è una posizione sul piano orizzontale del circuito, in metri.
// Nel sistema del gioco Y è l'altezza: la pianta usa X e Z.
type Point struct {
	X float32 `json:"x"`
	Z float32 `json:"z"`
}

// TrackMap è la pianta del circuito ricostruita finora.
type TrackMap struct {
	Length int        `json:"length"` // metri
	Points []MapPoint `json:"points"` // in ordine di percorrenza; i tratti non ancora visti mancano
}

// MapPoint è un punto della pianta con la sua distanza dal traguardo: serve
// per colorare i settori dei commissari (bandiere gialle).
type MapPoint struct {
	Point
	D int `json:"d"` // metri dall'inizio del giro
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
	m := TrackMap{Length: o.length, Points: make([]MapPoint, 0, o.filled)}
	for i, p := range o.points {
		if o.seen[i] {
			m.Points = append(m.Points, MapPoint{Point: p, D: i * outlineStep})
		}
	}
	return m
}

// merge aggiunge i tratti di other non ancora visti.
func (o *outline) merge(other *outline) {
	for i, seen := range other.seen {
		if seen && !o.seen[i] {
			o.points[i] = other.points[i]
			o.seen[i] = true
			o.filled++
		}
	}
}

// complete dice se la pianta è abbastanza completa da essere salvata. Qualche
// tratto può mancare: se un'auto va veloce e i pacchetti sono radi, tra due
// posizioni può saltare un tratto da 10 metri.
func (o *outline) complete() bool {
	return o.filled*100 >= len(o.points)*98
}

// --- Piante salvate --------------------------------------------------------

// loadOutline legge la pianta salvata di un circuito. Restituisce nil se non
// c'è o se non corrisponde (lunghezza del giro o passo diversi).
func loadOutline(db *sql.DB, track telemetry.TrackID, length int) (*outline, error) {
	var savedLength, step int
	var data []byte
	err := db.QueryRow("SELECT track_length, step, points FROM track_maps WHERE track_id = ?", int(track)).
		Scan(&savedLength, &step, &data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if savedLength != length || step != outlineStep {
		return nil, nil
	}
	var points []*Point
	if err := json.Unmarshal(data, &points); err != nil {
		return nil, err
	}
	o := newOutline(length)
	if len(points) != len(o.points) {
		return nil, nil
	}
	for i, p := range points {
		if p != nil {
			o.points[i] = *p
			o.seen[i] = true
			o.filled++
		}
	}
	return o, nil
}

// saveOutline salva la pianta di un circuito, sostituendo quella precedente.
func saveOutline(db *sql.DB, track telemetry.TrackID, o *outline) error {
	points := make([]*Point, len(o.points))
	for i := range o.points {
		if o.seen[i] {
			points[i] = &o.points[i]
		}
	}
	data, err := json.Marshal(points)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		INSERT INTO track_maps (track_id, track_length, step, points, updated_at) VALUES (?, ?, ?, ?, datetime('now'))
		ON CONFLICT (track_id) DO UPDATE SET
			track_length = excluded.track_length, step = excluded.step,
			points = excluded.points, updated_at = excluded.updated_at`,
		int(track), o.length, outlineStep, data)
	return err
}

// round1 arrotonda ai 10 cm: più che sufficienti per una mappa e il JSON resta corto.
func round1(v float32) float32 {
	return float32(math.Round(float64(v)*10) / 10)
}
