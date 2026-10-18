package live

import (
	"fmt"
	"math"

	"f1-scoreboard/internal/telemetry"
)

// Il gioco segnala con i pacchetti Event quello che succede in pista: penalità,
// contatti, safety car, giri veloci. Il tracker li trasforma in messaggi della
// direzione gara. Sorpassi e contatti tra auto della IA sono decine per giro:
// si tengono solo quelli che coinvolgono un giocatore umano.

// Messaggi della direzione gara tenuti in memoria e inviati con lo stato.
const maxEvents = 40

// Tipi di messaggio della direzione gara: il frontend li usa per icone e colori.
const (
	KindPenalty    = "penalty"
	KindCollision  = "collision"
	KindOvertake   = "overtake"
	KindFastestLap = "fastestLap"
	KindRetirement = "retirement"
	KindSafetyCar  = "safetyCar"
	KindRedFlag    = "redFlag"
	KindFlag       = "flag" // partenza, bandiera a scacchi, vincitore
	KindDRS        = "drs"
	KindFlashback  = "flashback"
)

// RaceEvent è un messaggio della direzione gara.
type RaceEvent struct {
	ID   int     `json:"id"`   // progressivo nella sessione
	Time float32 `json:"time"` // secondi dall'inizio della sessione
	Lap  uint8   `json:"lap"`  // giro del primo in classifica, 0 prima della partenza
	Kind string  `json:"kind"`
	Text string  `json:"text"`
	Cars []int   `json:"cars"` // auto coinvolte
}

var retirementReasons = map[uint8]string{
	1: "ritirato", 3: "danni irreparabili", 4: "inattivo", 5: "giri insufficienti",
	6: "bandiera nera", 7: "bandiera rossa", 8: "guasto meccanico",
}

var drsDisabledReasons = []string{"pista bagnata", "safety car", "bandiera rossa", "giro minimo non raggiunto"}

// Infrazioni (Penalty.InfringementType), come nella specifica EA.
var infringements = []string{
	"blocco guidando lentamente", "blocco contromano", "retromarcia dalla griglia", "collisione grave",
	"collisione", "posizione non restituita dopo una collisione", "posizioni non restituite dopo una collisione",
	"taglio di curva con guadagno di tempo", "sorpasso tagliando una curva", "sorpassi tagliando una curva",
	"linea di uscita dai box oltrepassata", "bandiere blu ignorate", "bandiere gialle ignorate",
	"drive through ignorato", "troppi drive through", "drive through da scontare entro pochi giri",
	"drive through da scontare in questo giro", "eccesso di velocità in corsia box", "fermo troppo a lungo",
	"regolamento gomme ignorato", "troppe penalità", "troppi avvertimenti", "vicino alla squalifica",
	"regolamento gomme", "regolamento gomme", "giro annullato per taglio di curva", "giro annullato per uscita di pista",
	"uscita di pista con guadagno di tempo", "uscita di pista con guadagno di tempo", "uscita di pista con guadagno di tempo",
	"giro annullato per contatto col muro", "giro annullato per flashback", "giro annullato per rientro in pista",
	"corsia box bloccata", "partenza anticipata", "collisione con la safety car", "sorpasso in regime di safety car",
	"velocità eccessiva con la safety car", "velocità eccessiva con la virtual safety car",
	"troppo lento nel giro di formazione", "fermo nel giro di formazione", "guasto meccanico", "danni irreparabili",
	"troppo distante dalla safety car", "bandiera nera", "stop & go non scontato", "drive through non scontato",
	"sostituzione di componenti del motore", "sostituzione del cambio", "modifiche in parco chiuso", "penalità del campionato",
	"penalità per il riavvio", "guadagno di tempo irregolare", "pit stop obbligatorio", "",
}

// isHuman dice se l'auto i è guidata da una persona: uno dei giocatori in
// locale o, online, un'auto non controllata dalla IA.
func (t *Tracker) isHuman(i int) bool {
	if controllerOf(t.header, i) > 0 {
		return true
	}
	p := t.participants
	return p != nil && i < len(p.Cars) && !p.Cars[i].AIControlled
}

func (t *Tracker) carName(i int) string {
	if p := t.participants; p != nil && i < len(p.Cars) && p.Cars[i].Name != "" {
		return p.Cars[i].Name
	}
	return fmt.Sprintf("Auto %d", i+1)
}

// raceLap è il giro del primo in classifica.
func (t *Tracker) raceLap() uint8 {
	if t.laps == nil {
		return 0
	}
	for _, lap := range t.laps.Cars {
		if lap.CarPosition == 1 {
			return lap.CurrentLapNum
		}
	}
	return 0
}

// handleEvent aggiunge alla direzione gara il messaggio dell'evento, se è da mostrare.
func (t *Tracker) handleEvent(e *telemetry.Event) {
	if e.Code == telemetry.EventLightsOut {
		t.lightsOut = true
	}
	kind, text, cars := t.describe(e)
	if text == "" {
		return
	}
	if cars == nil {
		cars = []int{}
	}
	t.nextEvent++
	t.events = append(t.events, RaceEvent{
		ID: t.nextEvent, Time: e.SessionTime, Lap: t.raceLap(), Kind: kind, Text: text, Cars: cars,
	})
	if len(t.events) > maxEvents {
		t.events = append(t.events[:0:0], t.events[len(t.events)-maxEvents:]...)
	}
}

// describe restituisce tipo, testo e auto coinvolte del messaggio. Testo
// vuoto se l'evento non va mostrato.
func (t *Tracker) describe(e *telemetry.Event) (kind, text string, cars []int) {
	name := func(idx uint8) string { return t.carName(int(idx)) }
	switch d := e.Details.(type) {
	case telemetry.FastestLap:
		return KindFastestLap, fmt.Sprintf("Giro più veloce: %s %s", name(d.VehicleIdx), formatLap(d.LapTime)), []int{int(d.VehicleIdx)}
	case telemetry.Retirement:
		reason, ok := retirementReasons[d.Reason]
		if !ok {
			return "", "", nil // arrivato, sessione saltata o simulata: non è un ritiro
		}
		return KindRetirement, fmt.Sprintf("Ritiro: %s (%s)", name(d.VehicleIdx), reason), []int{int(d.VehicleIdx)}
	case telemetry.DRSDisabled:
		text := "DRS disabilitato"
		if int(d.Reason) < len(drsDisabledReasons) {
			text += " (" + drsDisabledReasons[d.Reason] + ")"
		}
		return KindDRS, text, nil
	case telemetry.RaceWinner:
		return KindFlag, "Vince " + name(d.VehicleIdx), []int{int(d.VehicleIdx)}
	case telemetry.Penalty:
		return t.describePenalty(d)
	case telemetry.DriveThroughServed:
		return KindPenalty, name(d.VehicleIdx) + " ha scontato il drive through", []int{int(d.VehicleIdx)}
	case telemetry.StopGoServed:
		return KindPenalty, name(d.VehicleIdx) + " ha scontato lo stop & go", []int{int(d.VehicleIdx)}
	case telemetry.Overtake:
		a, b := int(d.OvertakingVehicleIdx), int(d.BeingOvertakenVehicleIdx)
		if !t.isHuman(a) && !t.isHuman(b) {
			return "", "", nil
		}
		return KindOvertake, fmt.Sprintf("%s supera %s", name(d.OvertakingVehicleIdx), name(d.BeingOvertakenVehicleIdx)), []int{a, b}
	case telemetry.Collision:
		a, b := int(d.Vehicle1Idx), int(d.Vehicle2Idx)
		if !t.isHuman(a) && !t.isHuman(b) {
			return "", "", nil
		}
		return KindCollision, fmt.Sprintf("Contatto tra %s e %s", name(d.Vehicle1Idx), name(d.Vehicle2Idx)), []int{a, b}
	case telemetry.SafetyCar:
		return describeSafetyCar(d)
	case telemetry.Flashback:
		return KindFlashback, "Flashback", nil
	}
	switch e.Code {
	case telemetry.EventLightsOut:
		return KindFlag, "Semaforo spento: via!", nil
	case telemetry.EventChequeredFlag:
		return KindFlag, "Bandiera a scacchi", nil
	case telemetry.EventDRSEnabled:
		return KindDRS, "DRS abilitato", nil
	case telemetry.EventRedFlag:
		return KindRedFlag, "Bandiera rossa", nil
	}
	return "", "", nil
}

func (t *Tracker) describePenalty(d telemetry.Penalty) (kind, text string, cars []int) {
	car := int(d.VehicleIdx)
	var what string
	switch d.PenaltyType {
	case 0:
		what = "drive through"
	case 1:
		what = "stop & go"
	case 2:
		what = "penalità in griglia"
	case 4:
		what = fmt.Sprintf("+%d s", d.Time)
	case 5:
		// Gli avvertimenti sono frequenti: si mostrano solo ai giocatori.
		if !t.isHuman(car) {
			return "", "", nil
		}
		what = "avvertimento"
	case 6:
		what = "squalifica"
	case 17:
		what = "bandiera nera"
	default:
		// Promemoria, giri annullati, ritiri (già segnalati a parte)…
		return "", "", nil
	}
	text = t.carName(car) + ": " + what
	if int(d.InfringementType) < len(infringements) && infringements[d.InfringementType] != "" {
		text += " per " + infringements[d.InfringementType]
	}
	cars = []int{car}
	if d.OtherVehicleIdx != 255 && int(d.OtherVehicleIdx) != car {
		text += " (con " + t.carName(int(d.OtherVehicleIdx)) + ")"
		cars = append(cars, int(d.OtherVehicleIdx))
	}
	return KindPenalty, text, cars
}

func describeSafetyCar(d telemetry.SafetyCar) (kind, text string, cars []int) {
	names := []string{"", "Safety car", "Virtual safety car", "Giro di formazione"}
	if d.Type == 0 || int(d.Type) >= len(names) {
		return "", "", nil
	}
	if d.Type == 3 {
		if d.EventType != 0 {
			return "", "", nil
		}
		return KindSafetyCar, "Giro di formazione", nil
	}
	phases := []string{"in pista", "rientra ai box", "rientrata", "si riparte"}
	if int(d.EventType) >= len(phases) {
		return "", "", nil
	}
	return KindSafetyCar, names[d.Type] + ": " + phases[d.EventType], nil
}

// formatLap scrive un tempo in secondi come 1:26.432.
func formatLap(seconds float32) string {
	ms := int(math.Round(float64(seconds) * 1000))
	return fmt.Sprintf("%d:%02d.%03d", ms/60000, ms/1000%60, ms%1000)
}

// eventsSince restituisce i messaggi con ID maggiore di id, dal più vecchio.
func (t *Tracker) eventsSince(id int) []RaceEvent {
	for i, e := range t.events {
		if e.ID > id {
			return t.events[i:]
		}
	}
	return nil
}
