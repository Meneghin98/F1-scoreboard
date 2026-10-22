package live

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"f1-scoreboard/internal/telemetry"
)

// Snapshot è lo stato della sessione in corso, così come arriva al frontend.
type Snapshot struct {
	Status   string       `json:"status"` // "live": telemetria in arrivo, "paused": sessione aperta senza dati recenti, "waiting": nessuna sessione
	Session  *SessionInfo `json:"session"`
	Drivers  []Driver     `json:"drivers"`
	Players  []PlayerCar  `json:"players"`  // auto dei giocatori (due in split-screen), in ordine di controller
	Finished bool         `json:"finished"` // classifica finale ricevuta
	Lineup   *Lineup      `json:"lineup"`   // chi guida le auto dei giocatori, solo in gara
	Events   []RaceEvent  `json:"events"`   // ultimi messaggi della direzione gara, dal più vecchio
	// Telemetry dice se il server riceve la telemetria (porta, errori, indirizzi
	// da impostare nel gioco). Solo nello stato live, non nei replay.
	Telemetry *TelemetryStatus `json:"telemetry,omitempty"`
	// DataVersion cambia a ogni modifica dei dati salvati: le pagine si
	// ricaricano quando la vedono cambiare (vedi Service.DataChanged).
	DataVersion int64 `json:"dataVersion,omitempty"`
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
	Sectors     [2]int `json:"sectors"`   // metri dal traguardo a cui iniziano il settore 2 e il 3, 0 se non ancora noti

	Forecast        []Forecast    `json:"forecast"`        // previsioni per il resto della sessione
	PitWindowIdeal  uint8         `json:"pitWindowIdeal"`  // giro ideale per la sosta, 0 se non previsto
	PitWindowLatest uint8         `json:"pitWindowLatest"` // ultimo giro utile per la sosta
	PitRejoin       uint8         `json:"pitRejoin"`       // posizione prevista al rientro dai box del giocatore principale
	MarshalZones    []MarshalZone `json:"marshalZones"`
}

// Forecast è il meteo previsto tra Offset minuti.
type Forecast struct {
	Offset    uint8 `json:"offset"` // minuti
	Weather   uint8 `json:"weather"`
	Rain      uint8 `json:"rain"` // probabilità di pioggia, %
	TrackTemp int8  `json:"trackTemp"`
	AirTemp   int8  `json:"airTemp"`
}

// MarshalZone è un settore dei commissari: va da Start all'inizio del successivo.
type MarshalZone struct {
	Start float32 `json:"start"` // frazione del giro (0..1)
	Flag  int8    `json:"flag"`  // -1 sconosciuta, 0 nessuna, 1 verde, 2 blu, 3 gialla
}

// Stint è un treno di gomme usato in gara.
type Stint struct {
	Tyre   string `json:"tyre"`   // S, M, H, I, W
	EndLap uint8  `json:"endLap"` // ultimo giro, 0 se ancora in uso
}

type Driver struct {
	CarIndex      int     `json:"carIndex"`
	Position      uint8   `json:"position"`
	Name          string  `json:"name"`
	Team          string  `json:"team"`
	RaceNumber    uint8   `json:"raceNumber"`
	Controller    int     `json:"controller"`  // 1 o 2 per le auto dei giocatori, 0 per le altre
	PlayerName    string  `json:"playerName"`  // pilota del segnapunti alla guida, se scelto
	PlayerColor   string  `json:"playerColor"` // suo colore ("#rrggbb"), vuoto se non scelto
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

	GridPosition uint8     `json:"gridPosition"`
	LapInvalid   bool      `json:"lapInvalid"`  // giro in corso annullato
	Warnings     uint8     `json:"warnings"`    // avvertimenti ricevuti
	CornerCuts   uint8     `json:"cornerCuts"`  // di cui per taglio di curva
	Unserved     uint8     `json:"unserved"`    // drive through e stop & go ancora da scontare
	SpeedTrap    float32   `json:"speedTrap"`   // velocità massima allo speed trap, km/h; 0 se non rilevata
	LastSectors  [3]uint32 `json:"lastSectors"` // settori dell'ultimo giro completato, ms
	BestSectors  [3]uint32 `json:"bestSectors"` // migliori settori personali, ms
	Stints       []Stint   `json:"stints"`      // gomme usate, dalla prima
	DRS          bool      `json:"drs"`         // ala mobile aperta
	Overtake     bool      `json:"overtake"`    // Overtake Mode (regolamento 2026) o ERS in modalità sorpasso
	BlueFlag     bool      `json:"blueFlag"`    // bandiera blu: sta per essere doppiato
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
	carTelemetry2  *telemetry.CarTelemetry2 // solo formato 2026
	damage         *telemetry.CarDamage
	classification *telemetry.FinalClassification
	motion         *telemetry.Motion
	setups         *telemetry.CarSetups
	outline        *outline
	history        map[uint8]*telemetry.SessionHistory // storico dei giri per auto
	tyreSets       map[uint8]*telemetry.TyreSets       // set di gomme, solo delle auto dei giocatori
	lapPositions   [][]uint8                           // posizione di ogni auto all'inizio di ogni giro: [giro][auto]
	lightsOut      bool                                // semaforo spento: la gara è partita
	events         []RaceEvent                         // ultimi maxEvents messaggi della direzione gara
	nextEvent      int                                 // ID dell'ultimo messaggio
}

// relevant sono i pacchetti usati dal tracker: gli altri non vengono nemmeno decodificati.
var relevant = map[telemetry.PacketID]bool{
	telemetry.PacketCarSetups:           true,
	telemetry.PacketTyreSets:            true,
	telemetry.PacketLapPositions:        true,
	telemetry.PacketSession:             true,
	telemetry.PacketParticipants:        true,
	telemetry.PacketLapData:             true,
	telemetry.PacketCarStatus:           true,
	telemetry.PacketCarTelemetry:        true,
	telemetry.PacketCarTelemetry2:       true,
	telemetry.PacketCarDamage:           true,
	telemetry.PacketFinalClassification: true,
	telemetry.PacketSessionHistory:      true,
	telemetry.PacketMotion:              true,
	telemetry.PacketEvent:               true,
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
	// I set di gomme arrivano un'auto alla volta (il primo byte è l'auto):
	// servono solo quelli dei giocatori.
	if h.PacketID == telemetry.PacketTyreSets &&
		(len(data) <= telemetry.HeaderSize || controllerOf(h, int(data[telemetry.HeaderSize])) == 0) {
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
	case *telemetry.CarTelemetry2:
		t.carTelemetry2 = p
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
	case *telemetry.Event:
		t.handleEvent(p)
	case *telemetry.SessionHistory:
		if t.history == nil {
			t.history = map[uint8]*telemetry.SessionHistory{}
		}
		t.history[p.CarIdx] = p
	case *telemetry.TyreSets:
		if t.tyreSets == nil {
			t.tyreSets = map[uint8]*telemetry.TyreSets{}
		}
		t.tyreSets[p.CarIdx] = p
	case *telemetry.CarSetups:
		t.setups = p
	case *telemetry.LapPositions:
		// Le posizioni arrivano a blocchi di 50 giri.
		for i, positions := range p.Positions {
			lap := int(p.LapStart) + i
			for len(t.lapPositions) <= lap {
				t.lapPositions = append(t.lapPositions, nil)
			}
			t.lapPositions[lap] = positions
		}
	}
}

// bestLap restituisce il miglior giro dell'auto i, dallo storico.
func (t *Tracker) bestLap(i int) uint32 {
	h := t.history[uint8(i)]
	if h == nil {
		return 0
	}
	if n := int(h.BestLapTimeLapNum); n >= 1 && n <= len(h.Laps) {
		return h.Laps[n-1].LapTimeMS
	}
	return 0
}

// sectors restituisce i settori dell'ultimo giro completato dall'auto i e i suoi migliori.
func (t *Tracker) sectors(i int) (last, best [3]uint32) {
	h := t.history[uint8(i)]
	if h == nil {
		return
	}
	for j := len(h.Laps) - 1; j >= 0; j-- {
		if l := h.Laps[j]; l.LapTimeMS > 0 {
			last = [3]uint32{l.Sector1TimeMS, l.Sector2TimeMS, l.Sector3TimeMS}
			break
		}
	}
	for s, n := range []uint8{h.BestSector1LapNum, h.BestSector2LapNum, h.BestSector3LapNum} {
		if n >= 1 && int(n) <= len(h.Laps) {
			l := h.Laps[n-1]
			best[s] = [3]uint32{l.Sector1TimeMS, l.Sector2TimeMS, l.Sector3TimeMS}[s]
		}
	}
	return
}

// stints restituisce le gomme usate dall'auto i, dallo storico.
func (t *Tracker) stints(i int) []Stint {
	stints := []Stint{}
	h := t.history[uint8(i)]
	if h == nil {
		return stints
	}
	for _, s := range h.TyreStints {
		end := s.EndLap
		if end == 255 {
			end = 0
		}
		stints = append(stints, Stint{Tyre: tyreLetter(s.VisualCompound), EndLap: end})
	}
	return stints
}

// Session restituisce l'ultimo pacchetto Session ricevuto, o nil.
func (t *Tracker) Session() *telemetry.Session { return t.session }

// Participants restituisce l'ultimo elenco dei piloti, o nil.
func (t *Tracker) Participants() *telemetry.Participants { return t.participants }

// Classification restituisce la classifica finale, se è già arrivata.
func (t *Tracker) Classification() *telemetry.FinalClassification { return t.classification }

// RaceStarted dice se la gara è partita. Se il server è stato avviato a gara
// in corso il semaforo non si è visto: basta che qualcuno sia al secondo giro.
func (t *Tracker) RaceStarted() bool {
	if t.lightsOut || t.classification != nil {
		return true
	}
	if t.laps != nil {
		for _, lap := range t.laps.Cars {
			if lap.CurrentLapNum >= 2 {
				return true
			}
		}
	}
	return false
}

// aero dice se l'auto i ha l'ala mobile aperta e se sta usando la potenza extra
// per sorpassare: con il regolamento 2026 è l'Overtake Mode, prima la modalità
// "sorpasso" dell'ERS.
func (t *Tracker) aero(i int) (drs, overtake bool) {
	if c := t.carTelemetry; c != nil && i < len(c.Cars) {
		drs = c.Cars[i].DRS
	}
	if c := t.carTelemetry2; c != nil && i < len(c.Cars) && c.Cars[i].Regulations2026 {
		return drs, c.Cars[i].OvertakeActive
	}
	if s := t.status; s != nil && i < len(s.Cars) {
		overtake = s.Cars[i].ERSDeployMode == ersOvertake
	}
	return drs, overtake
}

// ersOvertake è la modalità dell'ERS che dà tutta la potenza per sorpassare.
const ersOvertake = 3

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
	snap := emptySnapshot(status)
	snap.Finished = t.classification != nil
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
		info.PitWindowIdeal = s.PitStopWindowIdealLap
		info.PitWindowLatest = s.PitStopWindowLatestLap
		info.PitRejoin = s.PitStopRejoinPosition
		info.Forecast = []Forecast{}
		for _, f := range s.WeatherForecast {
			// Le previsioni coprono tutto il weekend: si tengono quelle di questa sessione.
			if f.SessionType == s.SessionType && f.TimeOffset > 0 {
				info.Forecast = append(info.Forecast, Forecast{
					Offset: f.TimeOffset, Weather: f.Weather, Rain: f.RainPercentage,
					TrackTemp: f.TrackTemperature, AirTemp: f.AirTemperature,
				})
			}
		}
		info.MarshalZones = make([]MarshalZone, len(s.MarshalZones))
		for i, z := range s.MarshalZones {
			info.MarshalZones[i] = MarshalZone{Start: round3(z.ZoneStart), Flag: z.ZoneFlag}
		}
	}
	if t.outline != nil {
		info.MapPoints = t.outline.filled
		info.Sectors = t.outline.sectorStarts()
	}
	snap.Session = info
	snap.Players = t.playerCars()
	snap.Events = append(snap.Events, t.events...)

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
			Controller:    controllerOf(t.header, i),
			Lap:           lap.CurrentLapNum,
			LastLapMS:     lap.LastLapTimeMS,
			BestLapMS:     t.bestLap(i),
			GapToLeaderMS: lap.DeltaToRaceLeaderMS,
			GapAheadMS:    lap.DeltaToCarInFrontMS,
			InPit:         lap.PitStatus != 0,
			PitStops:      lap.NumPitStops,
			PenaltiesSec:  lap.Penalties,
			Status:        lap.ResultStatus.String(),
			GridPosition:  lap.GridPosition,
			LapInvalid:    lap.CurrentLapInvalid,
			Warnings:      lap.TotalWarnings,
			CornerCuts:    lap.CornerCuttingWarns,
			Unserved:      lap.UnservedDriveThrus + lap.UnservedStopGos,
			Stints:        t.stints(i),
		}
		if lap.SpeedTrapFastestLap != 255 {
			d.SpeedTrap = round1(lap.SpeedTrapFastest)
		}
		d.LastSectors, d.BestSectors = t.sectors(i)
		if p := t.participants; p != nil && i < len(p.Cars) {
			d.Name = p.Cars[i].Name
			d.Team = p.Cars[i].TeamID.String()
			d.RaceNumber = p.Cars[i].RaceNumber
			d.AI = p.Cars[i].AIControlled
		}
		if s := t.status; s != nil && i < len(s.Cars) {
			d.Tyre = tyreLetter(s.Cars[i].VisualTyreCompound)
			d.TyreAge = s.Cars[i].TyresAgeLaps
			d.BlueFlag = s.Cars[i].VehicleFIAFlags == 2
		}
		d.DRS, d.Overtake = t.aero(i)
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

// emptySnapshot è lo stato senza dati: gli elenchi sono vuoti e non null, così
// il frontend non deve controllarli.
func emptySnapshot(status string) Snapshot {
	return Snapshot{Status: status, Drivers: []Driver{}, Players: []PlayerCar{}, Events: []RaceEvent{}}
}

func round3(v float32) float32 {
	return float32(math.Round(float64(v)*1000) / 1000)
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
