package live

import (
	"math"

	"f1-scoreboard/internal/telemetry"
)

// Capacità della batteria ERS nel gioco: serve per mostrare la carica in percentuale.
const ersCapacityJ = 4_000_000

// PlayerCar è lo stato dell'auto di un giocatore: guida, carburante, ERS, gomme e danni.
type PlayerCar struct {
	CarIndex   int `json:"carIndex"`
	Controller int `json:"controller"` // 1 o 2 (split-screen)

	// Guida (pacchetto CarTelemetry)
	Speed      uint16  `json:"speed"` // km/h
	Gear       int8    `json:"gear"`  // -1 retro, 0 folle
	RPM        uint16  `json:"rpm"`
	MaxRPM     uint16  `json:"maxRpm"`
	Throttle   float32 `json:"throttle"` // 0..1
	Brake      float32 `json:"brake"`    // 0..1
	DRS        bool    `json:"drs"`      // DRS aperto
	EngineTemp uint16  `json:"engineTemp"`

	// Stato (pacchetto CarStatus)
	DRSAllowed    bool    `json:"drsAllowed"`
	PitLimiter    bool    `json:"pitLimiter"`
	BrakeBias     uint8   `json:"brakeBias"` // % all'anteriore
	FuelMix       uint8   `json:"fuelMix"`   // 0 povera, 1 standard, 2 ricca, 3 massima
	FuelKg        float32 `json:"fuelKg"`
	FuelLaps      float32 `json:"fuelLaps"`      // margine in giri rispetto alla fine della gara
	ERSPercent    float32 `json:"ersPercent"`    // carica della batteria
	ERSMode       uint8   `json:"ersMode"`       // 0 nessuno, 1 medio, 2 hotlap, 3 sorpasso
	ERSDeployedMJ float32 `json:"ersDeployedMj"` // energia usata in questo giro
	ERSHarvestMJ  float32 `json:"ersHarvestMj"`  // energia recuperata in questo giro

	// Gomme e freni, nell'ordine anteriore sinistra, anteriore destra,
	// posteriore sinistra, posteriore destra (come si vedono dall'alto).
	Wheels [4]Wheel `json:"wheels"`

	// Danni in percentuale (pacchetto CarDamage)
	Damage CarDamage `json:"damage"`
}

type Wheel struct {
	SurfaceTemp uint8   `json:"surfaceTemp"` // °C
	InnerTemp   uint8   `json:"innerTemp"`   // °C
	Pressure    float32 `json:"pressure"`    // PSI
	Wear        float32 `json:"wear"`        // %
	BrakeTemp   uint16  `json:"brakeTemp"`   // °C
	BrakeDamage uint8   `json:"brakeDamage"` // %
}

type CarDamage struct {
	FrontWingLeft  uint8 `json:"frontWingLeft"`
	FrontWingRight uint8 `json:"frontWingRight"`
	RearWing       uint8 `json:"rearWing"`
	Floor          uint8 `json:"floor"`
	Diffuser       uint8 `json:"diffuser"`
	Sidepod        uint8 `json:"sidepod"`
	Gearbox        uint8 `json:"gearbox"`
	Engine         uint8 `json:"engine"`
	DRSFault       bool  `json:"drsFault"`
	ERSFault       bool  `json:"ersFault"`
}

// wheelOrder converte l'ordine del gioco (RL, RR, FL, FR) in FL, FR, RL, RR.
var wheelOrder = [4]int{2, 3, 0, 1}

// playerCars raccoglie i dati delle auto dei giocatori: una, o due in split-screen.
func (t *Tracker) playerCars() []PlayerCar {
	cars := []PlayerCar{}
	for c, idx := range []uint8{t.header.PlayerCarIndex, t.header.SecondaryPlayerCarIndex} {
		// L'indice vale 255 quando si è spettatori o non c'è un secondo giocatore.
		if idx == 255 {
			continue
		}
		if p := t.playerCar(int(idx)); p != nil {
			p.Controller = c + 1
			cars = append(cars, *p)
		}
	}
	return cars
}

// playerCar raccoglie i dati dell'auto i, o nil se non ce ne sono ancora.
func (t *Tracker) playerCar(i int) *PlayerCar {
	var (
		tel *telemetry.CarTelemetryData
		st  *telemetry.CarStatusData
		dmg *telemetry.CarDamageData
	)
	if t.carTelemetry != nil && i < len(t.carTelemetry.Cars) {
		tel = &t.carTelemetry.Cars[i]
	}
	if t.status != nil && i < len(t.status.Cars) {
		st = &t.status.Cars[i]
	}
	if t.damage != nil && i < len(t.damage.Cars) {
		dmg = &t.damage.Cars[i]
	}
	if tel == nil && st == nil && dmg == nil {
		return nil
	}

	p := &PlayerCar{CarIndex: i}
	if tel != nil {
		p.Speed, p.Gear, p.RPM = tel.Speed, tel.Gear, tel.EngineRPM
		p.Throttle, p.Brake = round2(tel.Throttle), round2(tel.Brake)
		p.DRS, p.EngineTemp = tel.DRS, tel.EngineTemperature
		for to, from := range wheelOrder {
			w := &p.Wheels[to]
			w.SurfaceTemp = tel.TyresSurfaceTemperature[from]
			w.InnerTemp = tel.TyresInnerTemperature[from]
			w.Pressure = round1(tel.TyresPressure[from])
			w.BrakeTemp = tel.BrakesTemperature[from]
		}
	}
	if st != nil {
		p.MaxRPM = st.MaxRPM
		p.DRSAllowed, p.PitLimiter = st.DRSAllowed, st.PitLimiter
		p.BrakeBias, p.FuelMix = st.FrontBrakeBias, st.FuelMix
		p.FuelKg, p.FuelLaps = round1(st.FuelInTank), round2(st.FuelRemainingLaps)
		p.ERSPercent = round1(st.ERSStoreEnergy / ersCapacityJ * 100)
		p.ERSMode = st.ERSDeployMode
		p.ERSDeployedMJ = round2(st.ERSDeployedThisLap / 1e6)
		p.ERSHarvestMJ = round2((st.ERSHarvestedThisLapMGUK + st.ERSHarvestedThisLapMGUH) / 1e6)
	}
	if dmg != nil {
		for to, from := range wheelOrder {
			w := &p.Wheels[to]
			w.Wear = round1(dmg.TyresWear[from])
			w.BrakeDamage = dmg.BrakesDamage[from]
		}
		p.Damage = CarDamage{
			FrontWingLeft: dmg.FrontLeftWing, FrontWingRight: dmg.FrontRightWing, RearWing: dmg.RearWing,
			Floor: dmg.Floor, Diffuser: dmg.Diffuser, Sidepod: dmg.Sidepod,
			Gearbox: dmg.GearBox, Engine: dmg.Engine, DRSFault: dmg.DRSFault, ERSFault: dmg.ERSFault,
		}
	}
	return p
}

func round2(v float32) float32 {
	return float32(math.Round(float64(v)*100) / 100)
}
