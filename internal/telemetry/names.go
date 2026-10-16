package telemetry

import "fmt"

// SessionType è il tipo di sessione (uguale nei formati 2025 e 2026).
type SessionType uint8

const (
	SessionUnknown SessionType = iota
	SessionPractice1
	SessionPractice2
	SessionPractice3
	SessionShortPractice
	SessionQualifying1
	SessionQualifying2
	SessionQualifying3
	SessionShortQualifying
	SessionOneShotQualifying
	SessionSprintShootout1
	SessionSprintShootout2
	SessionSprintShootout3
	SessionShortSprintShootout
	SessionOneShotSprintShootout
	SessionRace
	SessionRace2
	SessionRace3
	SessionTimeTrial
)

var sessionNames = [...]string{
	"Sconosciuta", "Prove libere 1", "Prove libere 2", "Prove libere 3", "Prove libere brevi",
	"Qualifica 1", "Qualifica 2", "Qualifica 3", "Qualifica breve", "Qualifica secca",
	"Sprint Shootout 1", "Sprint Shootout 2", "Sprint Shootout 3", "Sprint Shootout breve", "Sprint Shootout secco",
	"Gara", "Gara 2", "Gara 3", "Prova a tempo",
}

func (s SessionType) String() string {
	if int(s) < len(sessionNames) {
		return sessionNames[s]
	}
	return fmt.Sprintf("Sessione %d", uint8(s))
}

// IsRace dice se la sessione è una gara (comprese le sprint).
func (s SessionType) IsRace() bool {
	return s == SessionRace || s == SessionRace2 || s == SessionRace3
}

// TrackID identifica il circuito; -1 = sconosciuto.
type TrackID int8

var trackNames = map[TrackID]string{
	0: "Melbourne", 2: "Shanghai", 3: "Sakhir (Bahrain)", 4: "Catalunya", 5: "Monaco", 6: "Montreal",
	7: "Silverstone", 9: "Hungaroring", 10: "Spa", 11: "Monza", 12: "Singapore", 13: "Suzuka",
	14: "Abu Dhabi", 15: "Texas", 16: "Brazil", 17: "Austria", 19: "Mexico", 20: "Baku (Azerbaijan)",
	26: "Zandvoort", 27: "Imola", 29: "Jeddah", 30: "Miami", 31: "Las Vegas", 32: "Losail",
	39: "Silverstone (Reverse)", 40: "Austria (Reverse)", 41: "Zandvoort (Reverse)",
	42: "Madrid", // solo con il DLC 2026
}

func (t TrackID) String() string {
	if name, ok := trackNames[t]; ok {
		return name
	}
	return fmt.Sprintf("Circuito %d", int8(t))
}

// TeamID identifica la squadra. Nel formato 2026 i valori superano 255.
type TeamID uint16

var teamNames = map[TeamID]string{
	0: "Mercedes", 1: "Ferrari", 2: "Red Bull Racing", 3: "Williams", 4: "Aston Martin", 5: "Alpine",
	6: "RB", 7: "Haas", 8: "McLaren", 9: "Sauber",
	41: "F1 Generic", 104: "F1 Custom Team", 129: "Konnersport", 142: "APXGP '24", 154: "APXGP '25",
	155: "Konnersport '24",
	158: "Art GP '24", 159: "Campos '24", 160: "Rodin Motorsport '24", 161: "AIX Racing '24", 162: "DAMS '24",
	163: "Hitech '24", 164: "MP Motorsport '24", 165: "Prema '24", 166: "Trident '24",
	167: "Van Amersfoort Racing '24", 168: "Invicta '24",
	185: "Mercedes '24", 186: "Ferrari '24", 187: "Red Bull Racing '24", 188: "Williams '24",
	189: "Aston Martin '24", 190: "Alpine '24", 191: "RB '24", 192: "Haas '24", 193: "McLaren '24", 194: "Sauber '24",
	// Solo formato 2026.
	465: "Art GP '25", 466: "Campos '25", 467: "Rodin Motorsport '25", 468: "AIX Racing '25", 469: "DAMS '25",
	470: "Hitech '25", 471: "MP Motorsport '25", 472: "Prema '25", 473: "Trident '25",
	474: "Van Amersfoort Racing '25", 475: "Invicta '25",
	476: "Mercedes '26", 477: "Ferrari '26", 478: "Red Bull Racing '26", 479: "Williams '26",
	480: "Aston Martin '26", 481: "Alpine '26", 482: "RB '26", 483: "Haas '26", 484: "McLaren '26",
	485: "Audi '26", 486: "Cadillac '26",
	489: "Art GP '26", 490: "Campos '26", 491: "Rodin Motorsport '26", 492: "AIX Racing '26", 493: "DAMS '26",
	494: "Hitech '26", 495: "MP Motorsport '26", 496: "Prema '26", 497: "Trident '26",
	498: "Van Amersfoort Racing '26", 499: "Invicta '26",
}

func (t TeamID) String() string {
	if name, ok := teamNames[t]; ok {
		return name
	}
	return fmt.Sprintf("Team %d", uint16(t))
}

// ResultStatus è lo stato di un'auto nella sessione.
type ResultStatus uint8

const (
	ResultInvalid ResultStatus = iota
	ResultInactive
	ResultActive
	ResultFinished
	ResultDidNotFinish
	ResultDisqualified
	ResultNotClassified
	ResultRetired
)

var resultNames = [...]string{
	"Non valido", "Inattivo", "In pista", "Arrivato", "Non arrivato", "Squalificato", "Non classificato", "Ritirato",
}

func (s ResultStatus) String() string {
	if int(s) < len(resultNames) {
		return resultNames[s]
	}
	return fmt.Sprintf("Stato %d", uint8(s))
}
