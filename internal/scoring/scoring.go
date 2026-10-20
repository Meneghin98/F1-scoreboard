// Package scoring contiene il sistema di punteggio F1 2025, le regole dei
// tornei e il calcolo della classifica.
package scoring

import (
	"cmp"
	"database/sql/driver"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Points elenca i punti assegnati dal 1° posto in giù.
// Dal 2025 non esiste più il punto per il giro veloce: nei tornei si può
// rimettere, insieme ai punti per la pole (vedi Rules).
var Points = map[string][]int{
	"race":   {25, 18, 15, 12, 10, 8, 6, 4, 2, 1},
	"sprint": {8, 7, 6, 5, 4, 3, 2, 1},
}

// Modalità di punteggio dei tornei (Rules.Mode).
const (
	// ModeAbsolute: contano le posizioni d'arrivo, IA compresa, come nel gioco:
	// chi arriva 11° dietro a 10 auto della IA non prende punti.
	ModeAbsolute = "absolute"
	// ModePlayers: in ogni gara le posizioni si ricalcolano tra i soli piloti
	// del segnapunti, come se le auto della IA non ci fossero.
	ModePlayers = "players"
)

// Rules sono le regole di punteggio di un torneo. Il valore zero è il
// regolamento F1 2025: posizioni d'arrivo e nessun bonus.
type Rules struct {
	Mode             string `json:"points_mode"`        // ModeAbsolute ("" vale lo stesso) o ModePlayers
	PolePoints       int    `json:"pole_points"`        // al primo della qualifica
	FastestLapPoints int    `json:"fastest_lap_points"` // al giro più veloce di un Gran Premio, se non si ritira
	// Punti per posizione di Gran Premi e sprint, solo in modalità ModePlayers:
	// tra pochi piloti la tabella F1 può non andare bene. nil vale quella F1.
	RacePoints   PointsTable `json:"race_points"`
	SprintPoints PointsTable `json:"sprint_points"`
}

// Table restituisce i punti per posizione di un tipo di gara con queste regole.
func (r Rules) Table(raceType string) []int {
	if r.Mode == ModePlayers {
		if raceType == "race" && r.RacePoints != nil {
			return r.RacePoints
		}
		if raceType == "sprint" && r.SprintPoints != nil {
			return r.SprintPoints
		}
	}
	return Points[raceType]
}

// PointsTable sono i punti dal 1° posto in giù. Nel database è il testo
// "10,6,3"; il testo vuoto è nil, cioè la tabella F1.
type PointsTable []int

func (p *PointsTable) Scan(src any) error {
	var text string
	switch v := src.(type) {
	case string:
		text = v
	case []byte:
		text = string(v)
	default:
		return fmt.Errorf("tabella dei punti: tipo %T non valido", src)
	}
	*p = nil
	if text == "" {
		return nil
	}
	for _, field := range strings.Split(text, ",") {
		n, err := strconv.Atoi(field)
		if err != nil {
			return fmt.Errorf("tabella dei punti %q: %w", text, err)
		}
		*p = append(*p, n)
	}
	return nil
}

func (p PointsTable) Value() (driver.Value, error) {
	fields := make([]string, len(p))
	for i, n := range p {
		fields[i] = strconv.Itoa(n)
	}
	return strings.Join(fields, ","), nil
}

// Counts dice se un tipo di gara dà punti con queste regole: Gran Premi e
// sprint sempre, le qualifiche solo se la pole vale dei punti, le prove libere
// mai. Le gare che non danno punti mostrano i piazzamenti ma restano fuori dalla classifica.
func (r Rules) Counts(raceType string) bool {
	if raceType == "qualifying" {
		return r.PolePoints > 0
	}
	_, ok := Points[raceType]
	return ok
}

// PointsFor restituisce i punti della tabella per un piazzamento, senza bonus.
// position è nil se il pilota non è classificato.
func (r Rules) PointsFor(raceType string, position *int, dnf bool) int {
	if dnf || position == nil || *position < 1 {
		return 0
	}
	table := r.Table(raceType)
	if *position > len(table) {
		return 0
	}
	return table[*position-1]
}

// Result è un singolo risultato di gara, con i dati del pilota che servono alla classifica.
type Result struct {
	RaceID     int64
	RaceType   string
	PlayerID   int64
	Name       string
	Team       *string
	Color      string
	Position   *int
	DNF        bool
	FastestLap bool   // giro più veloce della gara, IA compresa (dalla registrazione o segnato a mano)
	BestLapMS  *int64 // miglior giro, solo dalle registrazioni
}

// Score è come conta un risultato con le regole del torneo.
type Score struct {
	// Position è la posizione che dà i punti: quella d'arrivo o, in modalità
	// ModePlayers, quella tra i soli piloti. nil se il pilota non è classificato.
	Position *int
	Points   int // compresi i bonus
	// FastestLap dice se è il giro più veloce della gara. In modalità
	// ModePlayers è il migliore dei piloti, anche se la IA è andata più forte.
	FastestLap bool
	Bonus      int // punti per la pole o per il giro veloce, già compresi in Points
}

// ScoreRace calcola i punti dei risultati di una gara con le regole del torneo.
// I risultati sono tutti della stessa gara: servono insieme perché le posizioni
// tra i piloti e il giro più veloce dipendono anche dagli altri. Il punteggio
// i-esimo è quello del risultato i-esimo.
func ScoreRace(results []Result, rules Rules) []Score {
	scores := make([]Score, len(results))
	if len(results) == 0 {
		return scores
	}
	raceType := results[0].RaceType

	// Posizioni che danno i punti: i classificati in ordine d'arrivo, numerati
	// da 1 in modalità ModePlayers. I ritirati restano senza posizione.
	var finished []int
	for i, r := range results {
		if !r.DNF && r.Position != nil && *r.Position > 0 {
			finished = append(finished, i)
		}
	}
	slices.SortStableFunc(finished, func(a, b int) int { return cmp.Compare(*results[a].Position, *results[b].Position) })
	for rank, i := range finished {
		position := *results[i].Position
		if rules.Mode == ModePlayers {
			position = rank + 1
		}
		scores[i].Position = &position
	}

	fastest := fastestLaps(results, rules.Mode)
	for i, r := range results {
		sc := &scores[i]
		sc.FastestLap = fastest[i]
		if raceType == "qualifying" && sc.Position != nil && *sc.Position == 1 {
			sc.Bonus += rules.PolePoints
		}
		// Il giro veloce vale punti solo nei Gran Premi, e solo a chi arriva.
		if raceType == "race" && sc.FastestLap && sc.Position != nil {
			sc.Bonus += rules.FastestLapPoints
		}
		sc.Points = rules.PointsFor(raceType, sc.Position, r.DNF) + sc.Bonus
	}
	return scores
}

// fastestLaps dice chi ha fatto il giro più veloce della gara. In modalità
// ModePlayers è il miglior tempo tra i risultati; senza tempi (come nelle gare
// inserite a mano) vale il giro veloce segnato, come in modalità ModeAbsolute.
func fastestLaps(results []Result, mode string) []bool {
	var best int64
	if mode == ModePlayers {
		for _, r := range results {
			if r.BestLapMS != nil && *r.BestLapMS > 0 && (best == 0 || *r.BestLapMS < best) {
				best = *r.BestLapMS
			}
		}
	}
	out := make([]bool, len(results))
	for i, r := range results {
		if best > 0 {
			out[i] = r.BestLapMS != nil && *r.BestLapMS == best
		} else {
			out[i] = r.FastestLap
		}
	}
	return out
}

// RaceScore è il risultato di un pilota in una gara, come appare in classifica.
type RaceScore struct {
	Points   int  `json:"points"`   // compresi i bonus
	Position *int `json:"position"` // posizione che dà i punti: tra i soli piloti in modalità ModePlayers
	DNF      bool `json:"dnf"`
	Bonus    int  `json:"bonus"` // punti della pole (nelle qualifiche) o del giro veloce (nei Gran Premi)
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
	Poles    int                 `json:"poles"`       // qualifiche vinte
	Fastest  int                 `json:"fastestLaps"` // giri più veloci nei Gran Premi
	PerRace  map[int64]RaceScore `json:"perRace"`

	positions []int // piazzamenti nei Gran Premi, usati per il countback
}

// Standings calcola la classifica di un torneo a partire da tutti i suoi
// risultati, con le sue regole: posizioni, vittorie, podi e giri veloci sono
// quelli che danno i punti (tra i soli piloti in modalità ModePlayers). Le gare
// che non danno punti (vedi Rules.Counts) restano fuori, ma le qualifiche
// vinte contano sempre come pole position.
func Standings(results []Result, rules Rules) []Standing {
	// I punti di un risultato dipendono dagli altri della stessa gara: prima si raggruppano.
	var raceIDs []int64
	byRace := map[int64][]Result{}
	for _, r := range results {
		if _, ok := byRace[r.RaceID]; !ok {
			raceIDs = append(raceIDs, r.RaceID)
		}
		byRace[r.RaceID] = append(byRace[r.RaceID], r)
	}

	byPlayer := map[int64]*Standing{}
	for _, raceID := range raceIDs {
		race := byRace[raceID]
		for i, sc := range ScoreRace(race, rules) {
			r := race[i]
			counts := rules.Counts(r.RaceType)
			pole := r.RaceType == "qualifying" && sc.Position != nil && *sc.Position == 1
			if !counts && !pole {
				continue
			}
			s, ok := byPlayer[r.PlayerID]
			if !ok {
				s = &Standing{PlayerID: r.PlayerID, Name: r.Name, Team: r.Team, Color: r.Color, PerRace: map[int64]RaceScore{}}
				byPlayer[r.PlayerID] = s
			}
			if pole {
				s.Poles++
			}
			if !counts {
				continue
			}

			s.Points += sc.Points
			s.PerRace[r.RaceID] = RaceScore{Points: sc.Points, Position: sc.Position, DNF: r.DNF, Bonus: sc.Bonus}

			// Vittorie, podi, giri veloci e countback considerano solo i Gran Premi, non le sprint.
			if r.RaceType != "race" {
				continue
			}
			s.Races++
			if sc.FastestLap {
				s.Fastest++
			}
			if sc.Position == nil {
				s.DNF++
				continue
			}
			s.positions = append(s.positions, *sc.Position)
			if *sc.Position == 1 {
				s.Wins++
			}
			if *sc.Position <= 3 {
				s.Podiums++
			}
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
