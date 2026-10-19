// Package telemetry decodifica i pacchetti UDP inviati da F1 25.
//
// Sono supportati due formati, scelti nel gioco con l'opzione "Formato UDP":
//   - 2025: F1 25 base, 22 auto, pacchetti 0-15;
//   - 2026: F1 25 con il DLC "2026 Season Pack", 24 auto e il pacchetto 16 (Car Telemetry 2).
//
// L'header è identico nei due formati; cambia il contenuto (e la dimensione) dei pacchetti.
// Tutti i valori sono little-endian e le struct sono "packed" (senza padding),
// quindi si leggono direttamente con encoding/binary.
package telemetry

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// Valori di Header.PacketFormat supportati.
const (
	Format2025 = 2025
	Format2026 = 2026
)

// SupportedFormat dice se il formato indicato nell'header è uno di quelli gestiti.
func SupportedFormat(format uint16) bool {
	return format == Format2025 || format == Format2026
}

// PacketID identifica il tipo di pacchetto (byte 6 dell'header).
type PacketID uint8

const (
	PacketMotion PacketID = iota
	PacketSession
	PacketLapData
	PacketEvent
	PacketParticipants
	PacketCarSetups
	PacketCarTelemetry
	PacketCarStatus
	PacketFinalClassification
	PacketLobbyInfo
	PacketCarDamage
	PacketSessionHistory
	PacketTyreSets
	PacketMotionEx
	PacketTimeTrial
	PacketLapPositions
	PacketCarTelemetry2 // solo formato 2026
)

var packetNames = [...]string{
	"Motion", "Session", "LapData", "Event", "Participants", "CarSetups", "CarTelemetry", "CarStatus",
	"FinalClassification", "LobbyInfo", "CarDamage", "SessionHistory", "TyreSets", "MotionEx", "TimeTrial", "LapPositions",
	"CarTelemetry2",
}

func (id PacketID) String() string {
	if int(id) < len(packetNames) {
		return packetNames[id]
	}
	return fmt.Sprintf("Sconosciuto(%d)", uint8(id))
}

// HeaderSize è la dimensione in byte dell'header comune a tutti i pacchetti.
const HeaderSize = 29

// Header è l'intestazione comune a tutti i pacchetti.
type Header struct {
	PacketFormat            uint16 // 2025 o 2026, vedi SupportedFormat
	GameYear                uint8  // ultime due cifre, es. 25
	GameMajorVersion        uint8  // versione del gioco, es. 1
	GameMinorVersion        uint8  // es. 18
	PacketVersion           uint8  // versione di questo tipo di pacchetto
	PacketID                PacketID
	SessionUID              uint64  // identificativo univoco della sessione
	SessionTime             float32 // secondi dall'inizio della sessione
	FrameIdentifier         uint32  // riparte da capo dopo un flashback
	OverallFrameIdentifier  uint32  // non riparte con i flashback
	PlayerCarIndex          uint8   // indice dell'auto del giocatore 1
	SecondaryPlayerCarIndex uint8   // indice dell'auto del giocatore 2 in split-screen, 255 se assente
}

var ErrShortPacket = errors.New("pacchetto troppo corto")

// ParseHeader legge l'header all'inizio di un pacchetto. Si chiama per ogni
// pacchetto (circa 200 al secondo, molti di più leggendo una registrazione):
// i campi si leggono uno per uno, senza binary.Read che usa la reflection.
func ParseHeader(packet []byte) (Header, error) {
	if len(packet) < HeaderSize {
		return Header{}, ErrShortPacket
	}
	le := binary.LittleEndian
	return Header{
		PacketFormat:            le.Uint16(packet[0:]),
		GameYear:                packet[2],
		GameMajorVersion:        packet[3],
		GameMinorVersion:        packet[4],
		PacketVersion:           packet[5],
		PacketID:                PacketID(packet[6]),
		SessionUID:              le.Uint64(packet[7:]),
		SessionTime:             math.Float32frombits(le.Uint32(packet[15:])),
		FrameIdentifier:         le.Uint32(packet[19:]),
		OverallFrameIdentifier:  le.Uint32(packet[23:]),
		PlayerCarIndex:          packet[27],
		SecondaryPlayerCarIndex: packet[28],
	}, nil
}

// EventCode restituisce il codice di 4 lettere di un pacchetto Event (es. "SSTA"
// per l'inizio sessione, "SEND" per la fine). ok è false per gli altri pacchetti.
func EventCode(packet []byte) (code string, ok bool) {
	if len(packet) < HeaderSize+4 || PacketID(packet[6]) != PacketEvent {
		return "", false
	}
	return string(packet[HeaderSize : HeaderSize+4]), true
}
