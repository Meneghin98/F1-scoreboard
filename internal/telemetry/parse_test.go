package telemetry

import (
	"encoding/binary"
	"errors"
	"testing"
)

// Dimensioni ufficiali dei pacchetti, dalle specifiche EA "Data Output from F1 25"
// (formato 2025) e "Data Output from F1 25: 2026 Season Pack" (formato 2026).
var officialSizes = map[uint16]map[PacketID]int{
	Format2025: {
		PacketMotion: 1349, PacketSession: 753, PacketLapData: 1285, PacketEvent: 45,
		PacketParticipants: 1284, PacketCarSetups: 1133, PacketCarTelemetry: 1352, PacketCarStatus: 1239,
		PacketFinalClassification: 1042, PacketLobbyInfo: 954, PacketCarDamage: 1041, PacketSessionHistory: 1460,
		PacketTyreSets: 231, PacketMotionEx: 273, PacketTimeTrial: 101, PacketLapPositions: 1131,
	},
	Format2026: {
		PacketMotion: 1325, PacketSession: 926, PacketLapData: 1399, PacketEvent: 45,
		PacketParticipants: 1470, PacketCarSetups: 1233, PacketCarTelemetry: 1448, PacketCarStatus: 1445,
		PacketFinalClassification: 1134, PacketLobbyInfo: 1062, PacketCarDamage: 1133, PacketSessionHistory: 1460,
		PacketTyreSets: 231, PacketMotionEx: 273, PacketTimeTrial: 104, PacketLapPositions: 1231,
		PacketCarTelemetry2: 269,
	},
}

// emptyPacket crea un pacchetto della dimensione indicata, con header valido e dati a zero.
func emptyPacket(t *testing.T, format uint16, id PacketID, size int) []byte {
	t.Helper()
	data := buildPacket(t, Header{PacketFormat: format, GameYear: 25, PacketID: id}, nil)
	return append(data, make([]byte, size-HeaderSize)...)
}

// Ogni decoder deve consumare esattamente i byte previsti dalla specifica:
// è il controllo principale per il formato 2026, di cui non abbiamo registrazioni.
func TestOfficialSizes(t *testing.T) {
	for format, sizes := range officialSizes {
		for id, size := range sizes {
			if _, err := parse(emptyPacket(t, format, id, size), true); err != nil {
				t.Errorf("formato %d, %s: %v", format, id, err)
			}
			if _, err := parse(emptyPacket(t, format, id, size-1), false); !errors.Is(err, ErrShortPacket) {
				t.Errorf("formato %d, %s di un byte più corto: errore %v, atteso ErrShortPacket", format, id, err)
			}
		}
	}
}

func TestParseRejects(t *testing.T) {
	if _, err := Parse(emptyPacket(t, 2024, PacketMotion, 1349)); !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("formato 2024: %v", err)
	}
	if _, err := Parse(emptyPacket(t, Format2025, PacketCarTelemetry2, 269)); !errors.Is(err, ErrUnknownPacket) {
		t.Errorf("Car Telemetry 2 nel formato 2025: %v", err)
	}
	if _, err := Parse(emptyPacket(t, Format2025, PacketID(42), 100)); !errors.Is(err, ErrUnknownPacket) {
		t.Errorf("pacchetto 42: %v", err)
	}
	// I byte in più (es. campi aggiunti da un aggiornamento del gioco) vengono ignorati.
	if _, err := Parse(emptyPacket(t, Format2025, PacketEvent, 50)); err != nil {
		t.Errorf("pacchetto più lungo: %v", err)
	}
}

// I test seguenti scrivono valori in punti precisi dei pacchetti 2026 per
// verificare la posizione dei campi che cambiano rispetto al 2025.

func TestParticipants2026WideIDs(t *testing.T) {
	data := emptyPacket(t, Format2026, PacketParticipants, 1470)
	data[HeaderSize] = 1  // numActiveCars
	car := HeaderSize + 1 // prima auto: aiControlled(1) driverId(2) networkId(2) teamId(2) ...
	binary.LittleEndian.PutUint16(data[car+1:], 300)
	binary.LittleEndian.PutUint16(data[car+5:], 485) // Audi '26
	copy(data[car+10:], "Pilota")                    // nome dopo myTeam, raceNumber, nationality

	p, err := parse(data, true)
	if err != nil {
		t.Fatal(err)
	}
	c := p.(*Participants).Cars[0]
	if c.DriverID != 300 || c.TeamID.String() != "Audi '26" || c.Name != "Pilota" {
		t.Fatalf("partecipante %+v", c)
	}
}

func TestCarStatus2026HarvestLimit(t *testing.T) {
	data := emptyPacket(t, Format2026, PacketCarStatus, 1445)
	// Nel 2026 il limite di recupero ERS sta tra ersHarvestedThisLapMGUH e ersDeployedThisLap.
	// tc..pitLimiter(5) carburante(12) rpm(4) maxGears+drsAllowed(2) drsDistance(2) gomme+flag(4)
	// potenze ed energia(12) deployMode(1) recupero MGU-K e MGU-H(8)
	const limitOffset = 5 + 12 + 4 + 2 + 2 + 4 + 12 + 1 + 8
	car := HeaderSize
	binary.LittleEndian.PutUint32(data[car+limitOffset:], 0x40800000)   // 4.0
	binary.LittleEndian.PutUint32(data[car+limitOffset+4:], 0x40000000) // 2.0

	p, err := parse(data, true)
	if err != nil {
		t.Fatal(err)
	}
	c := p.(*CarStatus).Cars[0]
	if c.ERSHarvestedLimitPerLap != 4 || c.ERSDeployedThisLap != 2 {
		t.Fatalf("limite %v, energia usata %v", c.ERSHarvestedLimitPerLap, c.ERSDeployedThisLap)
	}
}

func TestEventCollisionSeverity(t *testing.T) {
	for _, tt := range []struct {
		format   uint16
		severity uint8
	}{{Format2025, 0}, {Format2026, 2}} {
		data := emptyPacket(t, tt.format, PacketEvent, 45)
		copy(data[HeaderSize:], "COLL")
		data[HeaderSize+4], data[HeaderSize+5], data[HeaderSize+6] = 3, 7, 2

		p, err := parse(data, true)
		if err != nil {
			t.Fatal(err)
		}
		got := p.(*Event).Details.(Collision)
		if got != (Collision{Vehicle1Idx: 3, Vehicle2Idx: 7, Severity: tt.severity}) {
			t.Errorf("formato %d: %+v", tt.format, got)
		}
	}
}

func TestMinutesAndMilliseconds(t *testing.T) {
	data := emptyPacket(t, Format2025, PacketLapData, 1285)
	car := HeaderSize + 8                             // dopo lastLapTime e currentLapTime
	binary.LittleEndian.PutUint16(data[car:], 12_345) // sector1 ms
	data[car+2] = 1                                   // sector1 minuti

	p, err := parse(data, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.(*LapData).Cars[0].Sector1TimeMS; got != 72_345 {
		t.Fatalf("settore 1 = %d ms, attesi 72345", got)
	}
}

func TestNames(t *testing.T) {
	if SessionRace.String() != "Gara" || !SessionRace2.IsRace() || SessionQualifying1.IsRace() {
		t.Error("tipi di sessione")
	}
	if TrackID(11).String() != "Monza" || TrackID(-1).String() != "Circuito -1" {
		t.Error("circuiti")
	}
	if TeamID(1).String() != "Ferrari" || ResultFinished.String() != "Arrivato" {
		t.Error("team e stati")
	}
}
