package live

import (
	"cmp"
	"fmt"
	"slices"

	"f1-scoreboard/internal/telemetry"
)

// Snapshot è lo stato della sessione in corso, così come arriva al frontend.
type Snapshot struct {
	Status   string       `json:"status"` // "waiting": nessun dato recente, "live": telemetria in arrivo
	Session  *SessionInfo `json:"session"`
	Drivers  []Driver     `json:"drivers"`
	Player   *PlayerCar   `json:"player"`   // auto del giocatore, nil se non ci sono dati
	Finished bool         `json:"finished"` // classifica finale ricevuta
}

type SessionInfo struct {
	UID         string `json:"uid"`
	Format      uint16 `json:"format"`
	TrackID     int    `json:"trackId"`
	Track       string `json:"track"`
	SessionType uint8  `json:"sessionType"`
	Type        string `json:"type"`
	IsRace      bool   `json:"isRace"`
	TotalLaps   uint8  `json:"totalLaps"`
	CurrentLap  uint8  `json:"currentLap"` // giro del primo in classifica
	TimeLeft    uint16 `json:"timeLeft"`   // secondi
	Paused      bool   `json:"paused"`
	SafetyCar   uint8  `json:"safetyCar"` // 0 nessuna, 1 safety car, 2 virtual, 3 giro di formazione
	Weather     uint8  `json:"weather"`
	TrackTemp   int8   `json:"trackTemp"`
	AirTemp     int8   `json:"airTemp"`
	MapPoints   int    `json:"mapPoints"` // punti della pianta ricostruiti finora: quando cambia, il frontend la riscarica
}

type Driver struct {
	CarIndex      int     `json:"carIndex"`
	Position      uint8   `json:"position"`
	Name          string  `json:"name"`
	Team          string  `json:"team"`
	RaceNumber    uint8   `json:"raceNumber"`
	IsPlayer      bool    `json:"isPlayer"`
	AI            bool    `json:"ai"`
	Lap           uint8   `json:"lap"`
	LastLapMS     uint32  `json:"lastLapMs"`
	BestLapMS     uint32  `json:"bestLapMs"`
	GapToLeaderMS uint32  `json:"gapToLeaderMs"`
	GapAheadMS    uint32  `json:"gapAheadMs"`
	InPit         bool    `json:"inPit"`
	PitStops      uint8   `json:"pitStops"`
	PenaltiesSec  uint8   `json:"penaltiesSec"`
	Status        string  `json:"status"`
	Tyre          string  `json:"tyre"` // S, M, H, I, W
	TyreAge       uint8   `json:"tyreAge"`
	X             float32 `json:"x"` // posizione sulla pianta, in metri (vedi Point)
	Z             float32 `json:"z"`
}

// Tracker ricostruisce lo stato di una sessione dai pacchetti che riceve.
// Non sa da dove arrivano: dal gioco in diretta o da una registrazione.
// Non è sicuro per l'uso concorrente: chi lo usa deve proteggerlo.
type Tracker struct {
	header         telemetry.Header
	session        *telemetry.Session
	participants   *telemetry.Participants
	laps           *telemetry.LapData
	status         *telemetry.CarStatus
	carTelemetry   *telemetry.CarTelemetry
	damage         *telemetry.CarDamage
	classification *telemetry.FinalClassification
	motion         *telemetry.Motion
	outline        *outline
	bestLaps       map[uint8]uint32 // miglior giro per auto, dagli storici SessionHistory
}

// relevant sono i pacchetti usati dal tracker: gli altri non vengono nemmeno decodificati.
var relevant = map[telemetry.PacketID]bool{
	telemetry.PacketSession:             true,
	telemetry.PacketParticipants:        true,
	telemetry.PacketLapData:             true,
	telemetry.PacketCarStatus:           true,
	telemetry.PacketCarTelemetry:        true,
	telemetry.PacketCarDamage:           true,
	telemetry.PacketFinalClassification: true,
	telemetry.PacketSessionHistory:      true,
	telemetry.PacketMotion:              true,
}

// Handle aggiorna lo stato con un pacchetto. Se il pacchetto appartiene a una
// sessione diversa, lo stato della precedente viene scartato.
func (t *Tracker) Handle(h telemetry.Header, data []byte) {
	if h.SessionUID != t.header.SessionUID {
		*t = Tracker{}
	}
	t.header = h
	if !relevant[h.PacketID] {
		return
	}
	p, err := telemetry.Parse(data)
	if err != nil {
		return
	}
	switch p := p.(type) {
	case *telemetry.Session:
		t.session = p
	case *telemetry.Participants:
		t.participants = p
	case *telemetry.LapData:
		t.laps = p
	case *telemetry.CarStatus:
		t.status = p
	case *telemetry.CarTelemetry:
		t.carTelemetry = p
	case *telemetry.CarDamage:
		t.damage = p
	case *telemetry.FinalClassification:
		t.classification = p
	case *telemetry.Motion:
		t.motion = p
		if t.outline == nil && t.session != nil && t.session.TrackLength > 0 {
			t.outline = newOutline(int(t.session.TrackLength))
		}
		if t.outline != nil && t.laps != nil {
			t.outline.add(t.laps, p)
		}
	case *telemetry.SessionHistory:
		if t.bestLaps == nil {
			t.bestLaps = map[uint8]uint32{}
		}
		if n := int(p.BestLapTimeLapNum); n >= 1 && n <= len(p.Laps) {
			t.bestLaps[p.CarIdx] = p.Laps[n-1].LapTimeMS
		}
	}
}

// Session restituisce l'ultimo pacchetto Session ricevuto, o nil.
func (t *Tracker) Session() *telemetry.Session { return t.session }

// Participants restituisce l'ultimo elenco dei piloti, o nil.
func (t *Tracker) Participants() *telemetry.Participants { return t.participants }

// Classification restituisce la classifica finale, se è già arrivata.
func (t *Tracker) Classification() *telemetry.FinalClassification { return t.classification }

// seedOutline parte da una pianta già nota (salvata in una sessione
// precedente sullo stesso circuito); i tratti mancanti si completano dal vivo.
func (t *Tracker) seedOutline(o *outline) {
	if t.outline == nil || t.outline.length != o.length {
		t.outline = o
		return
	}
	t.outline.merge(o)
}

// TrackMap restituisce la pianta del circuito ricostruita finora, o nil se
// non è ancora iniziata.
func (t *Tracker) TrackMap() *TrackMap {
	if t.outline == nil {
		return nil
	}
	m := t.outline.trackMap()
	return &m
}

// Snapshot fotografa lo stato attuale. status è deciso da chi chiama, che sa
// da quanto tempo non arrivano pacchetti.
func (t *Tracker) Snapshot(status string) Snapshot {
	snap := Snapshot{Status: status, Drivers: []Driver{}, Finished: t.classification != nil}
	if t.header.SessionUID == 0 {
		return snap
	}

	info := &SessionInfo{UID: fmt.Sprintf("%016x", t.header.SessionUID), Format: t.header.PacketFormat, TrackID: -1}
	if s := t.session; s != nil {
		info.TrackID = int(s.TrackID)
		info.Track = s.TrackID.String()
		info.SessionType = uint8(s.SessionType)
		info.Type = s.SessionType.String()
		info.IsRace = s.SessionType.IsRace()
		info.TotalLaps = s.TotalLaps
		info.TimeLeft = s.SessionTimeLeft
		info.Paused = s.GamePaused
		info.SafetyCar = s.SafetyCarStatus
		info.Weather = s.Weather
		info.TrackTemp = s.TrackTemperature
		info.AirTemp = s.AirTemperature
	}
	if t.outline != nil {
		info.MapPoints = t.outline.filled
	}
	snap.Session = info
	snap.Player = t.playerCar()

	if t.laps == nil {
		return snap
	}
	for i, lap := range t.laps.Cars {
		if lap.ResultStatus < telemetry.ResultActive || lap.CarPosition == 0 {
			continue
		}
		d := Driver{
			CarIndex:      i,
			Position:      lap.CarPosition,
			Name:          fmt.Sprintf("Auto %d", i+1),
			IsPlayer:      i == int(t.header.PlayerCarIndex) || i == int(t.header.SecondaryPlayerCarIndex),
			Lap:           lap.CurrentLapNum,
			LastLapMS:     lap.LastLapTimeMS,
			BestLapMS:     t.bestLaps[uint8(i)],
			GapToLeaderMS: lap.DeltaToRaceLeaderMS,
			GapAheadMS:    lap.DeltaToCarInFrontMS,
			InPit:         lap.PitStatus != 0,
			PitStops:      lap.NumPitStops,
			PenaltiesSec:  lap.Penalties,
			Status:        lap.ResultStatus.String(),
		}
		if p := t.participants; p != nil && i < len(p.Cars) {
			d.Name = p.Cars[i].Name
			d.Team = p.Cars[i].TeamID.String()
			d.RaceNumber = p.Cars[i].RaceNumber
			d.AI = p.Cars[i].AIControlled
		}
		if s := t.status; s != nil && i < len(s.Cars) {
			d.Tyre = tyreLetter(s.Cars[i].VisualTyreCompound)
			d.TyreAge = s.Cars[i].TyresAgeLaps
		}
		if m := t.motion; m != nil && i < len(m.Cars) {
			pos := m.Cars[i].WorldPosition
			d.X, d.Z = round1(pos.X), round1(pos.Z)
		}
		if d.Position == 1 {
			info.CurrentLap = d.Lap
		}
		snap.Drivers = append(snap.Drivers, d)
	}
	slices.SortFunc(snap.Drivers, func(a, b Driver) int { return cmp.Compare(a.Position, b.Position) })
	return snap
}

// tyreLetter converte la mescola visiva nella lettera mostrata a schermo.
func tyreLetter(compound uint8) string {
	switch compound {
	case 16:
		return "S"
	case 17:
		return "M"
	case 18:
		return "H"
	case 7:
		return "I"
	case 8:
		return "W"
	}
	return ""
}
