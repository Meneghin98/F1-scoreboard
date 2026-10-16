package telemetry

// CarLap è lo stato del giro di un'auto. I tempi sono in millisecondi
// (i minuti inviati separatamente dal gioco sono già inclusi).
type CarLap struct {
	LastLapTimeMS       uint32
	CurrentLapTimeMS    uint32
	Sector1TimeMS       uint32
	Sector2TimeMS       uint32
	DeltaToCarInFrontMS uint32
	DeltaToRaceLeaderMS uint32
	LapDistance         float32 // metri dall'inizio del giro (negativo prima del traguardo)
	TotalDistance       float32 // metri dall'inizio della sessione
	SafetyCarDelta      float32 // secondi
	CarPosition         uint8
	CurrentLapNum       uint8
	PitStatus           uint8 // 0 in pista, 1 corsia box, 2 piazzola
	NumPitStops         uint8
	Sector              uint8 // 0 = settore 1, 1 = settore 2, 2 = settore 3
	CurrentLapInvalid   bool
	Penalties           uint8 // secondi di penalità accumulati
	TotalWarnings       uint8
	CornerCuttingWarns  uint8
	UnservedDriveThrus  uint8
	UnservedStopGos     uint8
	GridPosition        uint8
	DriverStatus        uint8 // 0 garage, 1 giro lanciato, 2 giro di rientro, 3 giro di uscita, 4 in pista
	ResultStatus        ResultStatus
	PitLaneTimerActive  bool
	PitLaneTimeMS       uint16
	PitStopTimerMS      uint16
	PitStopServePenalty bool
	SpeedTrapFastest    float32 // km/h
	SpeedTrapFastestLap uint8   // 255 = non ancora rilevata
}

// LapData (pacchetto 2) contiene lo stato del giro di tutte le auto.
type LapData struct {
	Header
	Cars              []CarLap
	TimeTrialPBCarIdx uint8 // 255 se assente
	TimeTrialRivalIdx uint8 // 255 se assente
}

func decodeLapData(h Header, r *reader, cars int) *LapData {
	p := &LapData{Header: h, Cars: make([]CarLap, cars)}
	for i := range p.Cars {
		c := &p.Cars[i]
		c.LastLapTimeMS = r.u32()
		c.CurrentLapTimeMS = r.u32()
		c.Sector1TimeMS = r.minutesMS()
		c.Sector2TimeMS = r.minutesMS()
		c.DeltaToCarInFrontMS = r.minutesMS()
		c.DeltaToRaceLeaderMS = r.minutesMS()
		c.LapDistance = r.f32()
		c.TotalDistance = r.f32()
		c.SafetyCarDelta = r.f32()
		c.CarPosition = r.u8()
		c.CurrentLapNum = r.u8()
		c.PitStatus = r.u8()
		c.NumPitStops = r.u8()
		c.Sector = r.u8()
		c.CurrentLapInvalid = r.flag()
		c.Penalties = r.u8()
		c.TotalWarnings = r.u8()
		c.CornerCuttingWarns = r.u8()
		c.UnservedDriveThrus = r.u8()
		c.UnservedStopGos = r.u8()
		c.GridPosition = r.u8()
		c.DriverStatus = r.u8()
		c.ResultStatus = ResultStatus(r.u8())
		c.PitLaneTimerActive = r.flag()
		c.PitLaneTimeMS = r.u16()
		c.PitStopTimerMS = r.u16()
		c.PitStopServePenalty = r.flag()
		c.SpeedTrapFastest = r.f32()
		c.SpeedTrapFastestLap = r.u8()
	}
	p.TimeTrialPBCarIdx = r.u8()
	p.TimeTrialRivalIdx = r.u8()
	return p
}

// LapHistory è un giro completato. Tempi in millisecondi.
type LapHistory struct {
	LapTimeMS     uint32
	Sector1TimeMS uint32
	Sector2TimeMS uint32
	Sector3TimeMS uint32
	ValidFlags    uint8 // bit 0 giro valido, bit 1-3 settori 1-3 validi
}

func (l LapHistory) Valid() bool { return l.ValidFlags&0x01 != 0 }

type TyreStint struct {
	EndLap         uint8 // 255 = stint in corso
	ActualCompound uint8
	VisualCompound uint8
}

// SessionHistory (pacchetto 11) contiene lo storico dei giri di un'auto. Il gioco
// invia a rotazione un pacchetto per ogni auto.
type SessionHistory struct {
	Header
	CarIdx            uint8
	BestLapTimeLapNum uint8
	BestSector1LapNum uint8
	BestSector2LapNum uint8
	BestSector3LapNum uint8
	Laps              []LapHistory
	TyreStints        []TyreStint
}

func decodeSessionHistory(h Header, r *reader) *SessionHistory {
	p := &SessionHistory{Header: h, CarIdx: r.u8()}
	laps := make([]LapHistory, 100)
	stints := make([]TyreStint, 8)
	numLaps := count(r.u8(), len(laps))
	numStints := count(r.u8(), len(stints))
	p.BestLapTimeLapNum = r.u8()
	p.BestSector1LapNum = r.u8()
	p.BestSector2LapNum = r.u8()
	p.BestSector3LapNum = r.u8()
	for i := range laps {
		laps[i] = LapHistory{
			LapTimeMS: r.u32(), Sector1TimeMS: r.minutesMS(), Sector2TimeMS: r.minutesMS(),
			Sector3TimeMS: r.minutesMS(), ValidFlags: r.u8(),
		}
	}
	for i := range stints {
		stints[i] = TyreStint{EndLap: r.u8(), ActualCompound: r.u8(), VisualCompound: r.u8()}
	}
	p.Laps = laps[:numLaps]
	p.TyreStints = stints[:numStints]
	return p
}

// LapPositions (pacchetto 15) contiene la posizione di ogni auto all'inizio di
// ciascun giro, a blocchi di 50 giri.
type LapPositions struct {
	Header
	LapStart  uint8     // indice (da 0) del primo giro di questo blocco
	Positions [][]uint8 // Positions[giro][auto] = posizione; 0 = nessun dato
}

func decodeLapPositions(h Header, r *reader, cars int) *LapPositions {
	p := &LapPositions{Header: h}
	numLaps := count(r.u8(), 50)
	p.LapStart = r.u8()
	positions := make([][]uint8, 50)
	for lap := range positions {
		positions[lap] = append([]uint8(nil), r.take(cars)...)
	}
	p.Positions = positions[:numLaps]
	return p
}
