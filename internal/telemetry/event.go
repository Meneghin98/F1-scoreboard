package telemetry

// Codici degli eventi.
const (
	EventSessionStarted      = "SSTA"
	EventSessionEnded        = "SEND"
	EventFastestLap          = "FTLP"
	EventRetirement          = "RTMT"
	EventDRSEnabled          = "DRSE"
	EventDRSDisabled         = "DRSD"
	EventTeamMateInPits      = "TMPT"
	EventChequeredFlag       = "CHQF"
	EventRaceWinner          = "RCWN"
	EventPenalty             = "PENA"
	EventSpeedTrap           = "SPTP"
	EventStartLights         = "STLG"
	EventLightsOut           = "LGOT"
	EventDriveThroughServed  = "DTSV"
	EventStopGoServed        = "SGSV"
	EventFlashback           = "FLBK"
	EventButtons             = "BUTN"
	EventRedFlag             = "RDFL"
	EventOvertake            = "OVTK"
	EventSafetyCar           = "SCAR"
	EventCollision           = "COLL"
	EventPartialModeEnabled  = "PMEN" // solo 2026
	EventPartialModeDisabled = "PMDI" // solo 2026
	EventOvertakeEnabled     = "OVEN" // solo 2026
	EventOvertakeDisabled    = "OVDI" // solo 2026
)

// Event (pacchetto 3) segnala un evento della sessione. Details contiene uno dei
// tipi qui sotto, in base al codice, oppure nil per gli eventi senza dettagli
// (inizio/fine sessione, bandiera a scacchi, semaforo spento, ecc.).
type Event struct {
	Header
	Code    string
	Details any
}

type FastestLap struct {
	VehicleIdx uint8
	LapTime    float32 // secondi
}

type Retirement struct {
	VehicleIdx uint8
	Reason     uint8 // 1 ritirato, 2 arrivato, 3 danni, 4 inattivo, 5 giri insufficienti, 6 bandiera nera, ...
}

type DRSDisabled struct {
	Reason uint8 // 0 pista bagnata, 1 safety car, 2 bandiera rossa, 3 giro minimo non raggiunto
}

type TeamMateInPits struct{ VehicleIdx uint8 }

type RaceWinner struct{ VehicleIdx uint8 }

type Penalty struct {
	PenaltyType      uint8
	InfringementType uint8
	VehicleIdx       uint8
	OtherVehicleIdx  uint8
	Time             uint8 // secondi
	LapNum           uint8
	PlacesGained     uint8
}

type SpeedTrap struct {
	VehicleIdx                 uint8
	Speed                      float32 // km/h
	IsOverallFastestInSession  bool
	IsDriverFastestInSession   bool
	FastestVehicleIdxInSession uint8
	FastestSpeedInSession      float32
}

type StartLights struct{ NumLights uint8 }

type DriveThroughServed struct{ VehicleIdx uint8 }

type StopGoServed struct {
	VehicleIdx uint8
	StopTime   float32 // secondi
}

type Flashback struct {
	FrameIdentifier uint32
	SessionTime     float32
}

type Buttons struct {
	Status uint32 // bit dei tasti premuti, vedi la specifica EA
}

type Overtake struct {
	OvertakingVehicleIdx     uint8
	BeingOvertakenVehicleIdx uint8
}

type SafetyCar struct {
	Type      uint8 // 0 nessuna, 1 safety car, 2 virtual, 3 giro di formazione
	EventType uint8 // 0 schierata, 1 rientro, 2 rientrata, 3 ripartenza
}

type Collision struct {
	Vehicle1Idx uint8
	Vehicle2Idx uint8
	Severity    uint8 // solo 2026: 0 lieve, 1 media, 2 grave
}

type PartialModeEnabled struct {
	Reason uint8 // 0 pista bagnata, 1 safety car, 2 bandiera rossa
}

// eventDetailsSize è la dimensione della union dei dettagli, uguale nei due formati.
const eventDetailsSize = 12

func decodeEvent(h Header, r *reader) *Event {
	p := &Event{Header: h, Code: r.str(4)}
	d := &reader{buf: r.take(eventDetailsSize), wide: r.wide}

	switch p.Code {
	case EventFastestLap:
		p.Details = FastestLap{VehicleIdx: d.u8(), LapTime: d.f32()}
	case EventRetirement:
		p.Details = Retirement{VehicleIdx: d.u8(), Reason: d.u8()}
	case EventDRSDisabled:
		p.Details = DRSDisabled{Reason: d.u8()}
	case EventTeamMateInPits:
		p.Details = TeamMateInPits{VehicleIdx: d.u8()}
	case EventRaceWinner:
		p.Details = RaceWinner{VehicleIdx: d.u8()}
	case EventPenalty:
		p.Details = Penalty{
			PenaltyType: d.u8(), InfringementType: d.u8(), VehicleIdx: d.u8(), OtherVehicleIdx: d.u8(),
			Time: d.u8(), LapNum: d.u8(), PlacesGained: d.u8(),
		}
	case EventSpeedTrap:
		p.Details = SpeedTrap{
			VehicleIdx: d.u8(), Speed: d.f32(), IsOverallFastestInSession: d.flag(),
			IsDriverFastestInSession: d.flag(), FastestVehicleIdxInSession: d.u8(), FastestSpeedInSession: d.f32(),
		}
	case EventStartLights:
		p.Details = StartLights{NumLights: d.u8()}
	case EventDriveThroughServed:
		p.Details = DriveThroughServed{VehicleIdx: d.u8()}
	case EventStopGoServed:
		p.Details = StopGoServed{VehicleIdx: d.u8(), StopTime: d.f32()}
	case EventFlashback:
		p.Details = Flashback{FrameIdentifier: d.u32(), SessionTime: d.f32()}
	case EventButtons:
		p.Details = Buttons{Status: d.u32()}
	case EventOvertake:
		p.Details = Overtake{OvertakingVehicleIdx: d.u8(), BeingOvertakenVehicleIdx: d.u8()}
	case EventSafetyCar:
		p.Details = SafetyCar{Type: d.u8(), EventType: d.u8()}
	case EventCollision:
		c := Collision{Vehicle1Idx: d.u8(), Vehicle2Idx: d.u8()}
		if d.wide {
			c.Severity = d.u8()
		}
		p.Details = c
	case EventPartialModeEnabled:
		p.Details = PartialModeEnabled{Reason: d.u8()}
	}
	return p
}
