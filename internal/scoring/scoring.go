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
	PolePoints       int    `json:"pole_points"`        // a chi parte primo in qualifica, IA compresa
	FastestLapPoints int    `json:"fastest_lap_points"` // al giro più veloce di un Gran Premio, IA compresa, se non si ritira
	// Preset è la tabella dei punti per posizione, solo in modalità ModePlayers:
	// tra pochi piloti la tabella F1 può non andare bene. Una chiave di Presets,
	// o PresetCustom per i punti di RacePoints e SprintPoints. "" vale PresetF1.
	Preset string `json:"points_preset"`
	// Punti scelti per Gran Premi e sprint, con PresetCustom. nil vale la tabella F1.
	RacePoints   PointsTable `json:"race_points"`
	SprintPoints PointsTable `json:"sprint_points"`
}

// Tabelle dei punti per posizione (Rules.Preset).
const (
	PresetUltimateChampion = "ultimate_champion"
	PresetF1               = "f1"
	PresetRankDown         = "rank_down"
	PresetAllOrNothing     = "all_or_nothing"
	PresetPushAndPay       = "push_and_pay"
	PresetCustom           = "custom" // i punti scelti a mano, in Rules.RacePoints e Rules.SprintPoints
)

// Table sono i punti per posizione di una gara.
type Table struct {
	Points []int // dal 1° posto in giù
	Others int   // a chi arriva dopo, a chi non è classificato e ai ritirati: anche negativi
	// WinBonus sono i punti in più a chi vince la gara davanti a tutti, IA compresa.
	WinBonus int
}

// Presets sono le tabelle dei punti già pronte: data una gara e il numero dei
// piloti che la corrono, restituiscono i suoi punti. Per aggiungerne una basta
// una voce qui e una in POINTS_PRESETS nel frontend.
var Presets = map[string]func(raceType string, drivers int) Table{
	// Un punto al primo tra i piloti e uno in più se vince la gara davanti alla IA.
	// Le sprint danno solo il punto della vittoria.
	PresetUltimateChampion: func(raceType string, _ int) Table {
		if raceType == "sprint" {
			return Table{WinBonus: 1}
		}
		return Table{Points: []int{1}, WinBonus: 1}
	},
	// I punti della F1.
	PresetF1: func(raceType string, _ int) Table { return Table{Points: Points[raceType]} },
	// Tanti punti quanti i piloti al primo, uno in meno a ogni posizione: con 4 piloti 4-3-2-1.
	PresetRankDown: func(_ string, drivers int) Table {
		points := make([]int, drivers)
		for i := range points {
			points[i] = drivers - i
		}
		return Table{Points: points}
	},
	// Solo il vincitore prende punti, quanti ne prende il vincitore in F1.
	PresetAllOrNothing: func(raceType string, _ int) Table { return Table{Points: Points[raceType][:1]} },
	// Il vincitore prende 2 punti, tutti gli altri ne perdono uno, ritirati compresi.
	PresetPushAndPay: func(string, int) Table { return Table{Points: []int{2}, Others: -1} },
}

// TableFor restituisce i punti per posizione di un tipo di gara con queste
// regole; drivers è il numero dei piloti della gara. Solo Gran Premi e sprint
// hanno una tabella: le qualifiche danno al massimo i punti della pole.
func (r Rules) TableFor(raceType string, drivers int) Table {
	if _, ok := Points[raceType]; !ok {
		return Table{}
	}
	if r.Mode != ModePlayers {
		return Table{Points: Points[raceType]}
	}
	if r.Preset == PresetCustom {
		if raceType == "race" && r.RacePoints != nil {
			return Table{Points: r.RacePoints}
		}
		if raceType == "sprint" && r.SprintPoints != nil {
			return Table{Points: r.SprintPoints}
		}
		return Table{Points: Points[raceType]}
	}
	preset, ok := Presets[r.Preset]
	if !ok {
		preset = Presets[PresetF1]
	}
	return preset(raceType, drivers)
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
// drivers è il numero dei piloti della gara, position è nil se il pilota non è classificato.
func (r Rules) PointsFor(raceType string, drivers int, position *int, dnf bool) int {
	table := r.TableFor(raceType, drivers)
	if dnf || position == nil || *position < 1 || *position > len(table.Points) {
		return table.Others
	}
	return table.Points[*position-1]
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
	FastestLap bool // giro più veloce della gara, IA compresa (dalla registrazione o segnato a mano)
}

// Score è come conta un risultato con le regole del torneo.
type Score struct {
	// Position è la posizione che dà i punti: quella d'arrivo o, in modalità
	// ModePlayers, quella tra i soli piloti. nil se il pilota non è classificato.
	Position *int
	Points   int // compresi i bonus
	// Pole e FastestLap dicono se il pilota ha fatto la pole o il giro più
	// veloce della gara, IA compresa, anche in modalità ModePlayers.
	Pole       bool
	FastestLap bool
	Bonus      int // punti per la pole o per il giro veloce, già compresi in Points
	WinBonus   int // punti per la vittoria davanti alla IA (vedi Table.WinBonus), già compresi in Points
}

// ScoreRace calcola i punti dei risultati di una gara con le regole del torneo.
// I risultati sono tutti della stessa gara: servono insieme perché le posizioni
// tra i piloti dipendono anche dagli altri. Il punteggio i-esimo è quello del
// risultato i-esimo.
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

	table := rules.TableFor(raceType, len(results))
	for i, r := range results {
		sc := &scores[i]
		// Vittoria, pole e giro veloce si guardano sulla gara vera, IA compresa,
		// non sulle posizioni tra i piloti.
		first := !r.DNF && r.Position != nil && *r.Position == 1
		sc.Pole = raceType == "qualifying" && first
		sc.FastestLap = r.FastestLap
		if sc.Pole {
			sc.Bonus += rules.PolePoints
		}
		// Il giro veloce vale punti solo nei Gran Premi, e solo a chi arriva.
		if raceType == "race" && sc.FastestLap && sc.Position != nil {
			sc.Bonus += rules.FastestLapPoints
		}
		if first {
			sc.WinBonus = table.WinBonus
		}
		sc.Points = rules.PointsFor(raceType, len(results), sc.Position, r.DNF) + sc.Bonus + sc.WinBonus
	}
	return scores
}

// RaceScore è il risultato di un pilota in una gara, come appare in classifica.
type RaceScore struct {
	Points   int  `json:"points"`   // compresi i bonus
	Position *int `json:"position"` // posizione che dà i punti: tra i soli piloti in modalità ModePlayers
	DNF      bool `json:"dnf"`
	Bonus    int  `json:"bonus"`     // punti della pole (nelle qualifiche) o del giro veloce (nei Gran Premi)
	WinBonus int  `json:"win_bonus"` // punti per la vittoria davanti alla IA, già compresi in Points
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
// risultati, con le sue regole: posizioni, vittorie e podi sono quelli che
// danno i punti (tra i soli piloti in modalità ModePlayers), pole e giri veloci
// sono quelli veri, IA compresa. Le gare che non danno punti (vedi Rules.Counts)
// restano fuori, ma le qualifiche vinte contano sempre come pole position.
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
			if !counts && !sc.Pole {
				continue
			}
			s, ok := byPlayer[r.PlayerID]
			if !ok {
				s = &Standing{PlayerID: r.PlayerID, Name: r.Name, Team: r.Team, Color: r.Color, PerRace: map[int64]RaceScore{}}
				byPlayer[r.PlayerID] = s
			}
			if sc.Pole {
				s.Poles++
			}
			if !counts {
				continue
			}

			s.Points += sc.Points
			s.PerRace[r.RaceID] = RaceScore{Points: sc.Points, Position: sc.Position, DNF: r.DNF, Bonus: sc.Bonus, WinBonus: sc.WinBonus}

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
