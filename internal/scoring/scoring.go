// Package scoring contiene il sistema di punteggio F1 2025 e il calcolo della classifica.
package scoring

import (
	"slices"
	"strings"
)

// Points elenca i punti assegnati dal 1° posto in giù.
// Dal 2025 non esiste più il punto per il giro veloce.
var Points = map[string][]int{
	"race":   {25, 18, 15, 12, 10, 8, 6, 4, 2, 1},
	"sprint": {8, 7, 6, 5, 4, 3, 2, 1},
}

// Counts dice se un tipo di gara conta per la classifica: prove libere
// ("practice") e qualifiche ("qualifying") mostrano i piazzamenti ma non danno punti.
func Counts(raceType string) bool {
	_, ok := Points[raceType]
	return ok
}

// PointsFor restituisce i punti per un piazzamento. position è nil se il pilota non è classificato.
func PointsFor(raceType string, position *int, dnf bool) int {
	if dnf || position == nil || *position < 1 {
		return 0
	}
	table := Points[raceType]
	if *position > len(table) {
		return 0
	}
	return table[*position-1]
}

// Result è un singolo risultato di gara, con i dati del pilota che servono alla classifica.
type Result struct {
	RaceID   int64
	RaceType string
	PlayerID int64
	Name     string
	Team     *string
	Color    string
	Position *int
	DNF      bool
}

// RaceScore è il risultato di un pilota in una gara, come appare in classifica.
type RaceScore struct {
	Points   int  `json:"points"`
	Position *int `json:"position"`
	DNF      bool `json:"dnf"`
}

// Standing è una riga della classifica di un torneo.
type Standing struct {
	Rank     int                 `json:"rank"`
	PlayerID int64               `json:"player_id"`
	Name     string              `json:"name"`
	Team     *string             `json:"team"`
	Color    string              `json:"color"` // colore del pilota, "#rrggbb"
	Points   int                 `json:"points"`
	Wins     int                 `json:"wins"`
	Podiums  int                 `json:"podiums"`
	Races    int                 `json:"races"`
	DNF      int                 `json:"dnf"`
	PerRace  map[int64]RaceScore `json:"perRace"`

	positions []int // piazzamenti nei Gran Premi, usati per il countback
}

// Standings calcola la classifica a partire da tutti i risultati di un torneo.
// Le sessioni che non danno punti (prove libere e qualifiche) non contano.
func Standings(results []Result) []Standing {
	byPlayer := map[int64]*Standing{}
	for _, r := range results {
		if !Counts(r.RaceType) {
			continue
		}
		s, ok := byPlayer[r.PlayerID]
		if !ok {
			s = &Standing{PlayerID: r.PlayerID, Name: r.Name, Team: r.Team, Color: r.Color, PerRace: map[int64]RaceScore{}}
			byPlayer[r.PlayerID] = s
		}

		points := PointsFor(r.RaceType, r.Position, r.DNF)
		s.Points += points
		s.PerRace[r.RaceID] = RaceScore{Points: points, Position: r.Position, DNF: r.DNF}

		// Vittorie, podi e countback considerano solo i Gran Premi, non le sprint.
		if r.RaceType != "race" {
			continue
		}
		s.Races++
		if r.DNF || r.Position == nil {
			s.DNF++
			continue
		}
		s.positions = append(s.positions, *r.Position)
		if *r.Position == 1 {
			s.Wins++
		}
		if *r.Position <= 3 {
			s.Podiums++
		}
	}

	standings := make([]Standing, 0, len(byPlayer))
	for _, s := range byPlayer {
		standings = append(standings, *s)
	}
	slices.SortFunc(standings, Compare)
	for i := range standings {
		standings[i].Rank = i + 1
	}
	return standings
}

// Compare ordina per punti; a parità vale il "countback" sui piazzamenti in gara
// (più vittorie, poi più secondi posti, ecc.), come nel regolamento F1.
// Restituisce un valore negativo se a va prima di b.
func Compare(a, b Standing) int {
	if a.Points != b.Points {
		return b.Points - a.Points
	}
	maxPos := 0
	for _, p := range slices.Concat(a.positions, b.positions) {
		maxPos = max(maxPos, p)
	}
	for p := 1; p <= maxPos; p++ {
		if diff := countOf(b.positions, p) - countOf(a.positions, p); diff != 0 {
			return diff
		}
	}
	return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
}

func countOf(list []int, value int) int {
	n := 0
	for _, v := range list {
		if v == value {
			n++
		}
	}
	return n
}
