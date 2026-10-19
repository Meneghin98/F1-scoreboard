package live

import (
	"strings"

	"f1-scoreboard/internal/telemetry"
)

// Track è un circuito come lo chiama il segnapunti.
type Track struct {
	ID   int    `json:"id"` // identificativo del gioco (telemetry.TrackID)
	Name string `json:"name"`
}

// Tracks sono i circuiti di F1 25 nell'ordine del calendario 2025, seguiti
// dalle varianti. È l'elenco unico dei nomi: il frontend lo propone per le
// gare inserite a mano (GET /api/tracks) e le gare create dalle registrazioni
// si chiamano allo stesso modo.
var Tracks = []Track{
	{0, "Australia – Melbourne"}, {2, "Cina – Shanghai"}, {13, "Giappone – Suzuka"}, {3, "Bahrain – Sakhir"},
	{29, "Arabia Saudita – Jeddah"}, {30, "Miami"}, {27, "Emilia-Romagna – Imola"}, {5, "Monaco"},
	{4, "Spagna – Barcellona"}, {6, "Canada – Montréal"}, {17, "Austria – Red Bull Ring"},
	{7, "Gran Bretagna – Silverstone"}, {10, "Belgio – Spa-Francorchamps"}, {9, "Ungheria – Hungaroring"},
	{26, "Olanda – Zandvoort"}, {11, "Italia – Monza"}, {20, "Azerbaijan – Baku"}, {12, "Singapore – Marina Bay"},
	{15, "USA – Austin"}, {19, "Messico – Città del Messico"}, {16, "Brasile – Interlagos"}, {31, "Las Vegas"},
	{32, "Qatar – Lusail"}, {14, "Abu Dhabi – Yas Marina"},
	{39, "Gran Bretagna – Silverstone (inverso)"}, {40, "Austria – Red Bull Ring (inverso)"},
	{41, "Olanda – Zandvoort (inverso)"}, {42, "Spagna – Madrid"},
}

// RaceTrackName restituisce il nome del circuito da usare per una gara del torneo.
func RaceTrackName(id telemetry.TrackID) string {
	for _, t := range Tracks {
		if t.ID == int(id) {
			return t.Name
		}
	}
	return id.String()
}

// TrackIDByName restituisce il circuito con quel nome (senza distinguere
// maiuscole e minuscole), nil se il nome non è tra quelli noti.
func TrackIDByName(name string) *int {
	name = strings.TrimSpace(name)
	for _, t := range Tracks {
		if strings.EqualFold(t.Name, name) {
			id := t.ID
			return &id
		}
	}
	return nil
}
