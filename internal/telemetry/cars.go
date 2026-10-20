package telemetry

// Gli array per ruota sono sempre nell'ordine: posteriore sinistra, posteriore
// destra, anteriore sinistra, anteriore destra.

type LiveryColour struct{ R, G, B uint8 }

// Participant descrive un pilota in sessione.
type Participant struct {
	AIControlled    bool
	DriverID        uint16 // 255 = giocatore umano; uint8 nel formato 2025
	NetworkID       uint16
	TeamID          TeamID
	MyTeam          bool
	RaceNumber      uint8
	Nationality     uint8
	Name            string // può essere generico se il giocatore nasconde il nome online
	YourTelemetry   uint8  // 0 riservata, 1 pubblica
	ShowOnlineNames bool
	TechLevel       uint16 // livello tecnico F1 World
	Platform        uint8  // 1 Steam, 3 PlayStation, 4 Xbox, 6 Origin, 255 sconosciuta
	LiveryColours   []LiveryColour
}

// Participants (pacchetto 4) elenca i piloti in sessione. Arriva ogni 5 secondi.
type Participants struct {
	Header
	NumActiveCars uint8
	Cars          []Participant
}

// NumCars restituisce quante posizioni dell'array sono occupate da un'auto
// della sessione. NumActiveCars non basta: cala quando un'auto si ritira, ma
// l'auto resta al suo posto e quelle dopo non si spostano. Le posizioni mai
// usate sono vuote, quindi si conta fino all'ultima auto con un nome.
func (p *Participants) NumCars() int {
	n := min(int(p.NumActiveCars), len(p.Cars))
	for i := len(p.Cars) - 1; i >= n; i-- {
		if p.Cars[i].Name != "" {
			return i + 1
		}
	}
	return n
}

func decodeParticipants(h Header, r *reader, cars int) *Participants {
	p := &Participants{Header: h, NumActiveCars: r.u8(), Cars: make([]Participant, cars)}
	for i := range p.Cars {
		c := &p.Cars[i]
		c.AIControlled = r.flag()
		c.DriverID = r.id()
		c.NetworkID = r.id()
		c.TeamID = TeamID(r.id())
		c.MyTeam = r.flag()
		c.RaceNumber = r.u8()
		c.Nationality = r.u8()
		c.Name = r.str(32)
		c.YourTelemetry = r.u8()
		c.ShowOnlineNames = r.flag()
		c.TechLevel = r.u16()
		c.Platform = r.u8()
		colours := make([]LiveryColour, 4)
		numColours := count(r.u8(), len(colours))
		for j := range colours {
			colours[j] = LiveryColour{R: r.u8(), G: r.u8(), B: r.u8()}
		}
		c.LiveryColours = colours[:numColours]
	}
	return p
}

// CarSetup è l'assetto di un'auto (vuoto per gli avversari con telemetria riservata).
type CarSetup struct {
	FrontWing             uint8
	RearWing              uint8
	OnThrottle            uint8 // differenziale in accelerazione, %
	OffThrottle           uint8
	FrontCamber           float32
	RearCamber            float32
	FrontToe              float32
	RearToe               float32
	FrontSuspension       uint8
	RearSuspension        uint8
	FrontAntiRollBar      uint8
	RearAntiRollBar       uint8
	FrontSuspensionHeight uint8
	RearSuspensionHeight  uint8
	BrakePressure         uint8      // %
	BrakeBias             uint8      // %
	EngineBraking         uint8      // %
	TyrePressures         [4]float32 // PSI, ordine delle ruote standard
	Ballast               uint8
	FuelLoad              float32
}

// CarSetups (pacchetto 5) contiene gli assetti di tutte le auto.
type CarSetups struct {
	Header
	Cars               []CarSetup
	NextFrontWingValue float32 // valore dell'ala anteriore dopo il prossimo pit stop (giocatore)
}

func decodeCarSetups(h Header, r *reader, cars int) *CarSetups {
	p := &CarSetups{Header: h, Cars: make([]CarSetup, cars)}
	for i := range p.Cars {
		c := &p.Cars[i]
		c.FrontWing = r.u8()
		c.RearWing = r.u8()
		c.OnThrottle = r.u8()
		c.OffThrottle = r.u8()
		c.FrontCamber = r.f32()
		c.RearCamber = r.f32()
		c.FrontToe = r.f32()
		c.RearToe = r.f32()
		c.FrontSuspension = r.u8()
		c.RearSuspension = r.u8()
		c.FrontAntiRollBar = r.u8()
		c.RearAntiRollBar = r.u8()
		c.FrontSuspensionHeight = r.u8()
		c.RearSuspensionHeight = r.u8()
		c.BrakePressure = r.u8()
		c.BrakeBias = r.u8()
		c.EngineBraking = r.u8()
		// Nella struct del gioco le pressioni sono in ordine RL, RR, FL, FR: coincide con l'ordine standard.
		c.TyrePressures = r.wheels()
		c.Ballast = r.u8()
		c.FuelLoad = r.f32()
	}
	p.NextFrontWingValue = r.f32()
	return p
}

// CarTelemetryData è la telemetria di guida di un'auto.
type CarTelemetryData struct {
	Speed                   uint16  // km/h
	Throttle                float32 // 0..1
	Steer                   float32 // -1 (sinistra) .. 1 (destra)
	Brake                   float32 // 0..1
	Clutch                  uint8   // 0..100
	Gear                    int8    // -1 retro, 0 folle, 1-8
	EngineRPM               uint16
	DRS                     bool
	RevLightsPercent        uint8
	RevLightsBitValue       uint16
	BrakesTemperature       [4]uint16  // °C
	TyresSurfaceTemperature [4]uint8   // °C
	TyresInnerTemperature   [4]uint8   // °C
	EngineTemperature       uint16     // °C (uint8 nel formato 2026)
	TyresPressure           [4]float32 // PSI
	SurfaceType             [4]uint8
}

// CarTelemetry (pacchetto 6) contiene la telemetria di guida di tutte le auto.
type CarTelemetry struct {
	Header
	Cars                         []CarTelemetryData
	MFDPanelIndex                uint8 // 255 = pannello chiuso
	MFDPanelIndexSecondaryPlayer uint8
	SuggestedGear                int8 // 0 = nessun suggerimento
}

func decodeCarTelemetry(h Header, r *reader, cars int) *CarTelemetry {
	p := &CarTelemetry{Header: h, Cars: make([]CarTelemetryData, cars)}
	for i := range p.Cars {
		c := &p.Cars[i]
		c.Speed = r.u16()
		c.Throttle = r.f32()
		c.Steer = r.f32()
		c.Brake = r.f32()
		c.Clutch = r.u8()
		c.Gear = r.i8()
		c.EngineRPM = r.u16()
		c.DRS = r.flag()
		c.RevLightsPercent = r.u8()
		c.RevLightsBitValue = r.u16()
		c.BrakesTemperature = [4]uint16{r.u16(), r.u16(), r.u16(), r.u16()}
		c.TyresSurfaceTemperature = r.wheelsU8()
		c.TyresInnerTemperature = r.wheelsU8()
		if r.wide {
			c.EngineTemperature = uint16(r.u8())
		} else {
			c.EngineTemperature = r.u16()
		}
		c.TyresPressure = r.wheels()
		c.SurfaceType = r.wheelsU8()
	}
	p.MFDPanelIndex = r.u8()
	p.MFDPanelIndexSecondaryPlayer = r.u8()
	p.SuggestedGear = r.i8()
	return p
}

// CarTelemetry2Data riguarda il regolamento 2026 (solo formato 2026).
type CarTelemetry2Data struct {
	ActiveAeroMode               uint8 // 0 modalità curva, 1 modalità rettilineo
	ActiveAeroAvailable          bool
	ActiveAeroActivationDistance uint16 // metri
	OvertakeAvailable            bool
	OvertakeActive               bool
	OvertakeActivationDistance   uint16 // metri; 0 = Overtake Mode non disponibile
	Regulations2026              bool   // l'auto segue il regolamento 2026
	DrivingWrongWay              bool
}

// CarTelemetry2 (pacchetto 16, solo formato 2026).
type CarTelemetry2 struct {
	Header
	Cars []CarTelemetry2Data
}

func decodeCarTelemetry2(h Header, r *reader, cars int) *CarTelemetry2 {
	p := &CarTelemetry2{Header: h, Cars: make([]CarTelemetry2Data, cars)}
	for i := range p.Cars {
		p.Cars[i] = CarTelemetry2Data{
			ActiveAeroMode: r.u8(), ActiveAeroAvailable: r.flag(), ActiveAeroActivationDistance: r.u16(),
			OvertakeAvailable: r.flag(), OvertakeActive: r.flag(), OvertakeActivationDistance: r.u16(),
			Regulations2026: r.flag(), DrivingWrongWay: r.flag(),
		}
	}
	return p
}

// CarStatusData è lo stato di un'auto: carburante, gomme, ERS.
type CarStatusData struct {
	TractionControl         uint8 // 0 off, 1 media, 2 completa
	AntiLockBrakes          bool
	FuelMix                 uint8 // 0 povera, 1 standard, 2 ricca, 3 massima
	FrontBrakeBias          uint8 // %
	PitLimiter              bool
	FuelInTank              float32 // kg
	FuelCapacity            float32
	FuelRemainingLaps       float32
	MaxRPM                  uint16
	IdleRPM                 uint16
	MaxGears                uint8
	DRSAllowed              bool
	DRSActivationDistance   uint16 // metri; 0 = DRS non disponibile
	ActualTyreCompound      uint8
	VisualTyreCompound      uint8 // 16 morbida, 17 media, 18 dura, 7 intermedia, 8 bagnato
	TyresAgeLaps            uint8
	VehicleFIAFlags         int8    // -1 sconosciuta, 0 nessuna, 1 verde, 2 blu, 3 gialla
	EnginePowerICE          float32 // W
	EnginePowerMGUK         float32 // W
	ERSStoreEnergy          float32 // J
	ERSDeployMode           uint8
	ERSHarvestedThisLapMGUK float32 // J
	ERSHarvestedThisLapMGUH float32 // J
	ERSHarvestedLimitPerLap float32 // J, solo formato 2026
	ERSDeployedThisLap      float32 // J
	NetworkPaused           bool
}

// CarStatus (pacchetto 7) contiene lo stato di tutte le auto.
type CarStatus struct {
	Header
	Cars []CarStatusData
}

func decodeCarStatus(h Header, r *reader, cars int) *CarStatus {
	p := &CarStatus{Header: h, Cars: make([]CarStatusData, cars)}
	for i := range p.Cars {
		c := &p.Cars[i]
		c.TractionControl = r.u8()
		c.AntiLockBrakes = r.flag()
		c.FuelMix = r.u8()
		c.FrontBrakeBias = r.u8()
		c.PitLimiter = r.flag()
		c.FuelInTank = r.f32()
		c.FuelCapacity = r.f32()
		c.FuelRemainingLaps = r.f32()
		c.MaxRPM = r.u16()
		c.IdleRPM = r.u16()
		c.MaxGears = r.u8()
		c.DRSAllowed = r.flag()
		c.DRSActivationDistance = r.u16()
		c.ActualTyreCompound = r.u8()
		c.VisualTyreCompound = r.u8()
		c.TyresAgeLaps = r.u8()
		c.VehicleFIAFlags = r.i8()
		c.EnginePowerICE = r.f32()
		c.EnginePowerMGUK = r.f32()
		c.ERSStoreEnergy = r.f32()
		c.ERSDeployMode = r.u8()
		c.ERSHarvestedThisLapMGUK = r.f32()
		c.ERSHarvestedThisLapMGUH = r.f32()
		if r.wide {
			c.ERSHarvestedLimitPerLap = r.f32()
		}
		c.ERSDeployedThisLap = r.f32()
		c.NetworkPaused = r.flag()
	}
	return p
}

// CarDamageData sono usura e danni di un'auto, in percentuale.
type CarDamageData struct {
	TyresWear      [4]float32
	TyresDamage    [4]uint8
	BrakesDamage   [4]uint8
	TyreBlisters   [4]uint8
	FrontLeftWing  uint8
	FrontRightWing uint8
	RearWing       uint8
	Floor          uint8
	Diffuser       uint8
	Sidepod        uint8
	DRSFault       bool
	ERSFault       bool
	GearBox        uint8
	Engine         uint8
	EngineMGUHWear uint8
	EngineESWear   uint8
	EngineCEWear   uint8
	EngineICEWear  uint8
	EngineMGUKWear uint8
	EngineTCWear   uint8
	EngineBlown    bool
	EngineSeized   bool
}

// CarDamage (pacchetto 10) contiene i danni di tutte le auto.
type CarDamage struct {
	Header
	Cars []CarDamageData
}

func decodeCarDamage(h Header, r *reader, cars int) *CarDamage {
	p := &CarDamage{Header: h, Cars: make([]CarDamageData, cars)}
	for i := range p.Cars {
		c := &p.Cars[i]
		c.TyresWear = r.wheels()
		c.TyresDamage = r.wheelsU8()
		c.BrakesDamage = r.wheelsU8()
		c.TyreBlisters = r.wheelsU8()
		c.FrontLeftWing = r.u8()
		c.FrontRightWing = r.u8()
		c.RearWing = r.u8()
		c.Floor = r.u8()
		c.Diffuser = r.u8()
		c.Sidepod = r.u8()
		c.DRSFault = r.flag()
		c.ERSFault = r.flag()
		c.GearBox = r.u8()
		c.Engine = r.u8()
		c.EngineMGUHWear = r.u8()
		c.EngineESWear = r.u8()
		c.EngineCEWear = r.u8()
		c.EngineICEWear = r.u8()
		c.EngineMGUKWear = r.u8()
		c.EngineTCWear = r.u8()
		c.EngineBlown = r.flag()
		c.EngineSeized = r.flag()
	}
	return p
}

type TyreSet struct {
	ActualCompound     uint8
	VisualCompound     uint8
	Wear               uint8 // %
	Available          bool
	RecommendedSession SessionType
	LifeSpan           uint8 // giri rimanenti
	UsableLife         uint8 // giri massimi consigliati
	LapDeltaTimeMS     int16 // differenza rispetto al set montato
	Fitted             bool
}

// TyreSets (pacchetto 12) elenca i set di gomme di un'auto (13 da asciutto + 7 da bagnato).
type TyreSets struct {
	Header
	CarIdx    uint8
	Sets      []TyreSet
	FittedIdx uint8
}

func decodeTyreSets(h Header, r *reader) *TyreSets {
	p := &TyreSets{Header: h, CarIdx: r.u8(), Sets: make([]TyreSet, 20)}
	for i := range p.Sets {
		p.Sets[i] = TyreSet{
			ActualCompound: r.u8(), VisualCompound: r.u8(), Wear: r.u8(), Available: r.flag(),
			RecommendedSession: SessionType(r.u8()), LifeSpan: r.u8(), UsableLife: r.u8(),
			LapDeltaTimeMS: r.i16(), Fitted: r.flag(),
		}
	}
	p.FittedIdx = r.u8()
	return p
}
