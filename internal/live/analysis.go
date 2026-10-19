package live

import "f1-scoreboard/internal/telemetry"

// Analysis riassume una sessione alla fine: posizioni giro per giro, gomme
// usate e tempi di ogni giro. Il replay la calcola sull'ultimo stato della
// registrazione e la mostra sotto la timeline.
type Analysis struct {
	Laps int           `json:"laps"` // giri con le posizioni: il primo è la griglia di partenza
	Cars []AnalysisCar `json:"cars"` // in ordine di classifica
}

type AnalysisCar struct {
	CarIndex    int       `json:"carIndex"`
	Position    uint8     `json:"position"`
	Name        string    `json:"name"`
	Team        string    `json:"team"`
	Controller  int       `json:"controller"` // 1 o 2 per le auto dei giocatori, 0 per le altre
	PlayerName  string    `json:"playerName"`
	PlayerColor string    `json:"playerColor"`
	Human       bool      `json:"human"`
	Status      string    `json:"status"`
	Positions   []int     `json:"positions"` // posizione all'inizio di ogni giro (0 = griglia); 0 se manca
	Stints      []Stint   `json:"stints"`
	Laps        []LapTime `json:"laps"` // giri completati
}

type LapTime struct {
	TimeMS  uint32    `json:"timeMs"`
	Sectors [3]uint32 `json:"sectors"`
	Valid   bool      `json:"valid"`
}

// Analysis riassume lo stato attuale. Nil se non ci sono ancora piloti.
func (t *Tracker) Analysis() *Analysis {
	drivers := t.Snapshot("live").Drivers
	if len(drivers) == 0 {
		return nil
	}
	a := &Analysis{Laps: len(t.lapPositions), Cars: make([]AnalysisCar, 0, len(drivers))}
	for _, d := range drivers {
		c := AnalysisCar{
			CarIndex: d.CarIndex, Position: d.Position, Name: d.Name, Team: d.Team, Controller: d.Controller,
			PlayerName: d.PlayerName, PlayerColor: d.PlayerColor, Human: t.isHuman(d.CarIndex), Status: d.Status, Stints: d.Stints,
			Positions: make([]int, len(t.lapPositions)), Laps: []LapTime{},
		}
		for lap, positions := range t.lapPositions {
			if d.CarIndex < len(positions) {
				c.Positions[lap] = int(positions[d.CarIndex])
			}
		}
		if h := t.history[uint8(d.CarIndex)]; h != nil {
			for _, l := range h.Laps {
				if l.LapTimeMS == 0 {
					continue // giro in corso
				}
				c.Laps = append(c.Laps, LapTime{
					TimeMS: l.LapTimeMS, Sectors: [3]uint32{l.Sector1TimeMS, l.Sector2TimeMS, l.Sector3TimeMS}, Valid: l.Valid(),
				})
			}
		}
		// Arrivato il primo, il gioco smette di aggiornare le posizioni: chi ha
		// finito la gara chiude il grafico con la posizione d'arrivo.
		if t.laps != nil && t.laps.Cars[d.CarIndex].ResultStatus == telemetry.ResultFinished && len(c.Laps) > 0 {
			n := len(c.Laps)
			for len(c.Positions) <= n {
				c.Positions = append(c.Positions, 0)
			}
			if c.Positions[n] == 0 {
				c.Positions[n] = int(d.Position)
			}
			a.Laps = max(a.Laps, len(c.Positions))
		}
		a.Cars = append(a.Cars, c)
	}
	for i := range a.Cars {
		for len(a.Cars[i].Positions) < a.Laps {
			a.Cars[i].Positions = append(a.Cars[i].Positions, 0)
		}
	}
	return a
}
