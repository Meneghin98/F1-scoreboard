package telemetry

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// buildPacket crea un pacchetto con l'header indicato seguito da payload.
func buildPacket(t *testing.T, h Header, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, h); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != HeaderSize {
		t.Fatalf("header di %d byte, attesi %d", buf.Len(), HeaderSize)
	}
	return append(buf.Bytes(), payload...)
}

func TestParseHeader(t *testing.T) {
	want := Header{
		PacketFormat: 2025, GameYear: 25, GameMajorVersion: 1, GameMinorVersion: 18, PacketVersion: 1,
		PacketID: PacketLapData, SessionUID: 0x1122334455667788, SessionTime: 12.5,
		FrameIdentifier: 300, OverallFrameIdentifier: 310, PlayerCarIndex: 19, SecondaryPlayerCarIndex: 255,
	}
	got, err := ParseHeader(buildPacket(t, want, make([]byte, 100)))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("header %+v, atteso %+v", got, want)
	}
	if got.PacketID.String() != "LapData" {
		t.Errorf("nome pacchetto %q", got.PacketID)
	}
}

func TestParseHeaderShort(t *testing.T) {
	if _, err := ParseHeader(make([]byte, HeaderSize-1)); err != ErrShortPacket {
		t.Fatalf("errore %v, atteso ErrShortPacket", err)
	}
}

func TestEventCode(t *testing.T) {
	event := buildPacket(t, Header{PacketID: PacketEvent}, []byte("SSTA"))
	if code, ok := EventCode(event); !ok || code != "SSTA" {
		t.Fatalf("EventCode = %q, %v", code, ok)
	}
	other := buildPacket(t, Header{PacketID: PacketMotion}, []byte("SSTA"))
	if _, ok := EventCode(other); ok {
		t.Fatal("un pacchetto Motion non ha codice evento")
	}
}

func TestPacketIDString(t *testing.T) {
	if PacketLapPositions.String() != "LapPositions" || PacketID(99).String() != "Sconosciuto(99)" {
		t.Fatal("nomi dei pacchetti errati")
	}
}

func TestFormat2026(t *testing.T) {
	// L'header del formato 2026 è identico: cambia solo il valore di PacketFormat.
	want := Header{PacketFormat: Format2026, GameYear: 25, PacketID: PacketCarTelemetry2, SessionUID: 42}
	got, err := ParseHeader(buildPacket(t, want, make([]byte, 240)))
	if err != nil {
		t.Fatal(err)
	}
	if got != want || got.PacketID.String() != "CarTelemetry2" {
		t.Fatalf("header %+v, atteso %+v", got, want)
	}
}

func TestSupportedFormat(t *testing.T) {
	for format, want := range map[uint16]bool{2024: false, 2025: true, 2026: true, 2027: false} {
		if SupportedFormat(format) != want {
			t.Errorf("SupportedFormat(%d) = %v", format, !want)
		}
	}
}
