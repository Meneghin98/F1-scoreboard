package telemetry

type MarshalZone struct {
	ZoneStart float32 // frazione del giro (0..1)
	ZoneFlag  int8    // -1 sconosciuto, 0 nessuna, 1 verde, 2 blu, 3 gialla
}

type WeatherForecastSample struct {
	SessionType            SessionType
	TimeOffset             uint8 // minuti
	Weather                uint8 // 0 sereno, 1 poco nuvoloso, 2 coperto, 3 pioggia leggera, 4 pioggia, 5 temporale
	TrackTemperature       int8  // °C
	TrackTemperatureChange int8  // 0 aumento, 1 calo, 2 stabile
	AirTemperature         int8
	AirTemperatureChange   int8
	RainPercentage         uint8
}

// Zone è un tratto di pista (zone DRS e di aerodinamica attiva, solo formato 2026).
type Zone struct {
	Start, End float32 // frazione del giro (0..1)
}

// Session (pacchetto 1) descrive la sessione in corso. Arriva due volte al secondo.
type Session struct {
	Header
	Weather           uint8
	TrackTemperature  int8 // °C
	AirTemperature    int8
	TotalLaps         uint8
	TrackLength       uint16 // metri
	SessionType       SessionType
	TrackID           TrackID
	Formula           uint8  // 0 F1 moderna, 1 F1 classica, 2 F2, 3 F1 generica, 4 Beta, 6 Esports, 8 F1 World, 9 F1 Elimination
	SessionTimeLeft   uint16 // secondi
	SessionDuration   uint16 // secondi
	PitSpeedLimit     uint8  // km/h
	GamePaused        bool
	IsSpectating      bool
	SpectatorCarIndex uint8
	SLIProNative      bool
	MarshalZones      []MarshalZone
	SafetyCarStatus   uint8 // 0 nessuna, 1 safety car, 2 virtual safety car, 3 giro di formazione
	NetworkGame       bool
	WeatherForecast   []WeatherForecastSample
	ForecastAccuracy  uint8 // 0 perfetta, 1 approssimata
	AIDifficulty      uint8 // 0-110

	SeasonLinkID  uint32 // identificativi per collegare le sessioni dello stesso weekend/stagione
	WeekendLinkID uint32
	SessionLinkID uint32

	PitStopWindowIdealLap  uint8
	PitStopWindowLatestLap uint8
	PitStopRejoinPosition  uint8

	SteeringAssist        uint8
	BrakingAssist         uint8
	GearboxAssist         uint8
	PitAssist             uint8
	PitReleaseAssist      uint8
	ERSAssist             uint8
	DRSAssist             uint8
	DynamicRacingLine     uint8
	DynamicRacingLineType uint8

	GameMode                        uint8
	RuleSet                         uint8
	TimeOfDay                       uint32 // minuti dalla mezzanotte
	SessionLength                   uint8
	SpeedUnitsLeadPlayer            uint8
	TemperatureUnitsLeadPlayer      uint8
	SpeedUnitsSecondaryPlayer       uint8
	TemperatureUnitsSecondaryPlayer uint8
	NumSafetyCarPeriods             uint8
	NumVirtualSafetyCarPeriods      uint8
	NumRedFlagPeriods               uint8
	EqualCarPerformance             uint8
	RecoveryMode                    uint8
	FlashbackLimit                  uint8
	SurfaceType                     uint8
	LowFuelMode                     uint8
	RaceStarts                      uint8
	TyreTemperature                 uint8
	PitLaneTyreSim                  uint8
	CarDamage                       uint8
	CarDamageRate                   uint8
	Collisions                      uint8
	CollisionsOffForFirstLapOnly    uint8
	MPUnsafePitRelease              uint8
	MPOffForGriefing                uint8
	CornerCuttingStringency         uint8
	ParcFermeRules                  uint8
	PitStopExperience               uint8
	SafetyCar                       uint8
	SafetyCarExperience             uint8
	FormationLap                    uint8
	FormationLapExperience          uint8
	RedFlags                        uint8
	AffectsLicenceLevelSolo         uint8
	AffectsLicenceLevelMP           uint8

	WeekendStructure        []SessionType // sessioni del weekend, in ordine
	Sector2LapDistanceStart float32       // metri
	Sector3LapDistanceStart float32

	// Solo formato 2026.
	ActiveAeroTrackStatus        uint8 // 0 completa, 1 parziale
	ActiveAeroZonesFull          []Zone
	ActiveAeroZonesPartial       []Zone
	DRSZones                     []Zone
	StartReactionTime            float32 // secondi
	AntiLockBrakesAssist         uint8
	TractionControlAssist        uint8
	DynamicRacingLineHiVis       uint8
	DynamicRacingLineColourBlind uint8
	RecurringRewindPrompt        uint8
}

func decodeSession(h Header, r *reader) *Session {
	p := &Session{Header: h}
	p.Weather = r.u8()
	p.TrackTemperature = r.i8()
	p.AirTemperature = r.i8()
	p.TotalLaps = r.u8()
	p.TrackLength = r.u16()
	p.SessionType = SessionType(r.u8())
	p.TrackID = TrackID(r.i8())
	p.Formula = r.u8()
	p.SessionTimeLeft = r.u16()
	p.SessionDuration = r.u16()
	p.PitSpeedLimit = r.u8()
	p.GamePaused = r.flag()
	p.IsSpectating = r.flag()
	p.SpectatorCarIndex = r.u8()
	p.SLIProNative = r.flag()

	zones := make([]MarshalZone, 21)
	numZones := count(r.u8(), len(zones))
	for i := range zones {
		zones[i] = MarshalZone{ZoneStart: r.f32(), ZoneFlag: r.i8()}
	}
	p.MarshalZones = zones[:numZones]

	p.SafetyCarStatus = r.u8()
	p.NetworkGame = r.flag()

	forecast := make([]WeatherForecastSample, 64)
	numForecast := count(r.u8(), len(forecast))
	for i := range forecast {
		forecast[i] = WeatherForecastSample{
			SessionType: SessionType(r.u8()), TimeOffset: r.u8(), Weather: r.u8(),
			TrackTemperature: r.i8(), TrackTemperatureChange: r.i8(),
			AirTemperature: r.i8(), AirTemperatureChange: r.i8(), RainPercentage: r.u8(),
		}
	}
	p.WeatherForecast = forecast[:numForecast]

	p.ForecastAccuracy = r.u8()
	p.AIDifficulty = r.u8()
	p.SeasonLinkID = r.u32()
	p.WeekendLinkID = r.u32()
	p.SessionLinkID = r.u32()
	p.PitStopWindowIdealLap = r.u8()
	p.PitStopWindowLatestLap = r.u8()
	p.PitStopRejoinPosition = r.u8()
	p.SteeringAssist = r.u8()
	p.BrakingAssist = r.u8()
	p.GearboxAssist = r.u8()
	p.PitAssist = r.u8()
	p.PitReleaseAssist = r.u8()
	p.ERSAssist = r.u8()
	p.DRSAssist = r.u8()
	p.DynamicRacingLine = r.u8()
	p.DynamicRacingLineType = r.u8()
	p.GameMode = r.u8()
	p.RuleSet = r.u8()
	p.TimeOfDay = r.u32()
	p.SessionLength = r.u8()
	p.SpeedUnitsLeadPlayer = r.u8()
	p.TemperatureUnitsLeadPlayer = r.u8()
	p.SpeedUnitsSecondaryPlayer = r.u8()
	p.TemperatureUnitsSecondaryPlayer = r.u8()
	p.NumSafetyCarPeriods = r.u8()
	p.NumVirtualSafetyCarPeriods = r.u8()
	p.NumRedFlagPeriods = r.u8()
	p.EqualCarPerformance = r.u8()
	p.RecoveryMode = r.u8()
	p.FlashbackLimit = r.u8()
	p.SurfaceType = r.u8()
	p.LowFuelMode = r.u8()
	p.RaceStarts = r.u8()
	p.TyreTemperature = r.u8()
	p.PitLaneTyreSim = r.u8()
	p.CarDamage = r.u8()
	p.CarDamageRate = r.u8()
	p.Collisions = r.u8()
	p.CollisionsOffForFirstLapOnly = r.u8()
	p.MPUnsafePitRelease = r.u8()
	p.MPOffForGriefing = r.u8()
	p.CornerCuttingStringency = r.u8()
	p.ParcFermeRules = r.u8()
	p.PitStopExperience = r.u8()
	p.SafetyCar = r.u8()
	p.SafetyCarExperience = r.u8()
	p.FormationLap = r.u8()
	p.FormationLapExperience = r.u8()
	p.RedFlags = r.u8()
	p.AffectsLicenceLevelSolo = r.u8()
	p.AffectsLicenceLevelMP = r.u8()

	weekend := make([]SessionType, 12)
	numSessions := count(r.u8(), len(weekend))
	for i := range weekend {
		weekend[i] = SessionType(r.u8())
	}
	p.WeekendStructure = weekend[:numSessions]

	p.Sector2LapDistanceStart = r.f32()
	p.Sector3LapDistanceStart = r.f32()

	if r.wide {
		p.ActiveAeroTrackStatus = r.u8()
		p.ActiveAeroZonesFull = r.zones(8)
		p.ActiveAeroZonesPartial = r.zones(8)
		p.DRSZones = r.zones(4)
		p.StartReactionTime = r.f32()
		p.AntiLockBrakesAssist = r.u8()
		p.TractionControlAssist = r.u8()
		p.DynamicRacingLineHiVis = r.u8()
		p.DynamicRacingLineColourBlind = r.u8()
		p.RecurringRewindPrompt = r.u8()
	}
	return p
}

// zones legge un contatore seguito da un array di capacity zone.
func (r *reader) zones(capacity int) []Zone {
	zones := make([]Zone, capacity)
	n := count(r.u8(), capacity)
	for i := range zones {
		zones[i] = Zone{Start: r.f32(), End: r.f32()}
	}
	return zones[:n]
}
