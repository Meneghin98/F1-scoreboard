// Package telemetry decodifica i pacchetti UDP inviati da F1 25 (formato 2025).
//
// Tutti i valori sono little-endian e le struct sono "packed" (senza padding),
// quindi si leggono direttamente con encoding/binary.
package telemetry

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

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
)

var packetNames = [...]string{
	"Motion", "Session", "LapData", "Event", "Participants", "CarSetups", "CarTelemetry", "CarStatus",
	"FinalClassification", "LobbyInfo", "CarDamage", "SessionHistory", "TyreSets", "MotionEx", "TimeTrial", "LapPositions",
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
	PacketFormat            uint16 // 2025
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

// ParseHeader legge l'header all'inizio di un pacchetto.
func ParseHeader(packet []byte) (Header, error) {
	var h Header
	if len(packet) < HeaderSize {
		return h, ErrShortPacket
	}
	err := binary.Read(bytes.NewReader(packet[:HeaderSize]), binary.LittleEndian, &h)
	return h, err
}

// EventCode restituisce il codice di 4 lettere di un pacchetto Event (es. "SSTA"
// per l'inizio sessione, "SEND" per la fine). ok è false per gli altri pacchetti.
func EventCode(packet []byte) (code string, ok bool) {
	if len(packet) < HeaderSize+4 || PacketID(packet[6]) != PacketEvent {
		return "", false
	}
	return string(packet[HeaderSize : HeaderSize+4]), true
}
