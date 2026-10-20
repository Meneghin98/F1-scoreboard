package telemetry

import (
	"errors"
	"fmt"
)

var (
	ErrUnsupportedFormat = errors.New("formato UDP non supportato (imposta 2025 o 2026 nel gioco)")
	ErrUnknownPacket     = errors.New("tipo di pacchetto sconosciuto")
)

// Packet è un pacchetto decodificato: *Motion, *Session, *LapData, *Event, ecc.
// Si distinguono con un type switch.
type Packet interface {
	PacketHeader() Header
}

// PacketHeader permette a ogni pacchetto (che incorpora Header) di soddisfare Packet.
func (h Header) PacketHeader() Header { return h }

// Vector3 è un vettore nello spazio del gioco.
type Vector3 struct{ X, Y, Z float32 }

// NumCars restituisce quante auto contengono gli array dei pacchetti nel formato indicato.
// Le posizioni oltre le auto della sessione sono vuote: vedi Participants.NumCars.
func NumCars(format uint16) int {
	if format == Format2026 {
		return 24
	}
	return 22
}

// Parse decodifica un pacchetto UDP di F1 25 nel formato 2025 o 2026.
// Eventuali byte in più in fondo al pacchetto vengono ignorati, così un
// aggiornamento del gioco che aggiunge campi non blocca la decodifica.
func Parse(data []byte) (Packet, error) {
	return parse(data, false)
}

// parse con strict=true rifiuta anche i pacchetti più lunghi del previsto:
// lo usano i test per verificare che ogni struct corrisponda alla specifica.
func parse(data []byte, strict bool) (Packet, error) {
	h, err := ParseHeader(data)
	if err != nil {
		return nil, err
	}
	if !SupportedFormat(h.PacketFormat) {
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedFormat, h.PacketFormat)
	}

	r := &reader{buf: data, off: HeaderSize, wide: h.PacketFormat == Format2026}
	cars := NumCars(h.PacketFormat)

	var p Packet
	switch h.PacketID {
	case PacketMotion:
		p = decodeMotion(h, r, cars)
	case PacketSession:
		p = decodeSession(h, r)
	case PacketLapData:
		p = decodeLapData(h, r, cars)
	case PacketEvent:
		p = decodeEvent(h, r)
	case PacketParticipants:
		p = decodeParticipants(h, r, cars)
	case PacketCarSetups:
		p = decodeCarSetups(h, r, cars)
	case PacketCarTelemetry:
		p = decodeCarTelemetry(h, r, cars)
	case PacketCarStatus:
		p = decodeCarStatus(h, r, cars)
	case PacketFinalClassification:
		p = decodeFinalClassification(h, r, cars)
	case PacketLobbyInfo:
		p = decodeLobbyInfo(h, r, cars)
	case PacketCarDamage:
		p = decodeCarDamage(h, r, cars)
	case PacketSessionHistory:
		p = decodeSessionHistory(h, r)
	case PacketTyreSets:
		p = decodeTyreSets(h, r)
	case PacketMotionEx:
		p = decodeMotionEx(h, r)
	case PacketTimeTrial:
		p = decodeTimeTrial(h, r)
	case PacketLapPositions:
		p = decodeLapPositions(h, r, cars)
	case PacketCarTelemetry2:
		if h.PacketFormat != Format2026 {
			return nil, fmt.Errorf("%w: %s nel formato %d", ErrUnknownPacket, h.PacketID, h.PacketFormat)
		}
		p = decodeCarTelemetry2(h, r, cars)
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnknownPacket, h.PacketID)
	}

	if r.short {
		return nil, fmt.Errorf("%s: %w (%d byte)", h.PacketID, ErrShortPacket, len(data))
	}
	if strict && r.off != len(data) {
		return nil, fmt.Errorf("%s: letti %d byte su %d", h.PacketID, r.off, len(data))
	}
	return p, nil
}
