package telemetry

// ClassificationEntry è il risultato finale di un'auto.
type ClassificationEntry struct {
	Position      uint8
	NumLaps       uint8
	GridPosition  uint8
	Points        uint8
	NumPitStops   uint8
	ResultStatus  ResultStatus
	ResultReason  uint8 // 1 ritirato, 2 arrivato, 3 danni, 4 inattivo, 5 giri insufficienti, 6 bandiera nera, ...
	BestLapTimeMS uint32
	TotalRaceTime float64 // secondi, senza penalità
	PenaltiesTime uint8   // secondi
	NumPenalties  uint8
	TyreStints    []TyreStint
}

// FinalClassification (pacchetto 8) arriva a fine sessione con la classifica definitiva.
// Cars è indicizzato per auto (come gli altri pacchetti), non per posizione.
type FinalClassification struct {
	Header
	NumCars uint8
	Cars    []ClassificationEntry
}

func decodeFinalClassification(h Header, r *reader, cars int) *FinalClassification {
	p := &FinalClassification{Header: h, NumCars: r.u8(), Cars: make([]ClassificationEntry, cars)}
	for i := range p.Cars {
		c := &p.Cars[i]
		c.Position = r.u8()
		c.NumLaps = r.u8()
		c.GridPosition = r.u8()
		c.Points = r.u8()
		c.NumPitStops = r.u8()
		c.ResultStatus = ResultStatus(r.u8())
		c.ResultReason = r.u8()
		c.BestLapTimeMS = r.u32()
		c.TotalRaceTime = r.f64()
		c.PenaltiesTime = r.u8()
		c.NumPenalties = r.u8()
		numStints := count(r.u8(), 8)
		var actual, visual, endLaps [8]uint8
		copy(actual[:], r.take(8))
		copy(visual[:], r.take(8))
		copy(endLaps[:], r.take(8))
		c.TyreStints = make([]TyreStint, numStints)
		for s := range c.TyreStints {
			c.TyreStints[s] = TyreStint{EndLap: endLaps[s], ActualCompound: actual[s], VisualCompound: visual[s]}
		}
	}
	return p
}

// LobbyPlayer è un giocatore nella lobby multiplayer.
type LobbyPlayer struct {
	AIControlled    bool
	TeamID          TeamID
	Nationality     uint8
	Platform        uint8
	Name            string
	CarNumber       uint8
	YourTelemetry   uint8 // 0 riservata, 1 pubblica
	ShowOnlineNames bool
	TechLevel       uint16
	ReadyStatus     uint8 // 0 non pronto, 1 pronto, 2 spettatore
}

// LobbyInfo (pacchetto 9) arriva nella lobby delle partite online.
type LobbyInfo struct {
	Header
	NumPlayers uint8
	Players    []LobbyPlayer
}

func decodeLobbyInfo(h Header, r *reader, cars int) *LobbyInfo {
	p := &LobbyInfo{Header: h, NumPlayers: r.u8(), Players: make([]LobbyPlayer, cars)}
	for i := range p.Players {
		p.Players[i] = LobbyPlayer{
			AIControlled: r.flag(), TeamID: TeamID(r.id()), Nationality: r.u8(), Platform: r.u8(),
			Name: r.str(32), CarNumber: r.u8(), YourTelemetry: r.u8(), ShowOnlineNames: r.flag(),
			TechLevel: r.u16(), ReadyStatus: r.u8(),
		}
	}
	return p
}

// TimeTrialDataSet è un giro di riferimento in prova a tempo.
type TimeTrialDataSet struct {
	CarIdx              uint8
	TeamID              TeamID
	LapTimeMS           uint32
	Sector1TimeMS       uint32
	Sector2TimeMS       uint32
	Sector3TimeMS       uint32
	TractionControl     uint8
	GearboxAssist       uint8
	AntiLockBrakes      bool
	EqualCarPerformance bool
	CustomSetup         bool
	Valid               bool
}

// TimeTrial (pacchetto 14) contiene i riferimenti della prova a tempo.
type TimeTrial struct {
	Header
	PlayerSessionBest TimeTrialDataSet
	PersonalBest      TimeTrialDataSet
	Rival             TimeTrialDataSet
}

func decodeTimeTrial(h Header, r *reader) *TimeTrial {
	return &TimeTrial{Header: h, PlayerSessionBest: r.timeTrialSet(), PersonalBest: r.timeTrialSet(), Rival: r.timeTrialSet()}
}

func (r *reader) timeTrialSet() TimeTrialDataSet {
	return TimeTrialDataSet{
		CarIdx: r.u8(), TeamID: TeamID(r.id()), LapTimeMS: r.u32(),
		Sector1TimeMS: r.u32(), Sector2TimeMS: r.u32(), Sector3TimeMS: r.u32(),
		TractionControl: r.u8(), GearboxAssist: r.u8(), AntiLockBrakes: r.flag(),
		EqualCarPerformance: r.flag(), CustomSetup: r.flag(), Valid: r.flag(),
	}
}
