package scoring

import "testing"

func pos(p int) *int { return &p }

func TestPointsFor(t *testing.T) {
	tests := []struct {
		raceType string
		position *int
		dnf      bool
		want     int
	}{
		{"race", pos(1), false, 25},
		{"race", pos(10), false, 1},
		{"race", pos(11), false, 0},
		{"sprint", pos(1), false, 8},
		{"sprint", pos(9), false, 0},
		{"race", pos(1), true, 0},
		{"race", nil, false, 0},
	}
	for _, tt := range tests {
		if got := PointsFor(tt.raceType, tt.position, tt.dnf); got != tt.want {
			t.Errorf("PointsFor(%s, %v, %v) = %d, atteso %d", tt.raceType, tt.position, tt.dnf, got, tt.want)
		}
	}
}

func TestStandingsCountback(t *testing.T) {
	// A e B finiscono a pari punti (25). Nei GP A ha un secondo posto e B no,
	// quindi A va davanti: la vittoria di B nella sprint non conta per il countback.
	results := []Result{
		{RaceID: 1, RaceType: "race", PlayerID: 1, Name: "A", Position: pos(2)},
		{RaceID: 1, RaceType: "race", PlayerID: 2, Name: "B", Position: pos(3)},
		{RaceID: 2, RaceType: "race", PlayerID: 2, Name: "B", Position: pos(9)},
		{RaceID: 3, RaceType: "sprint", PlayerID: 2, Name: "B", Position: pos(1)},
		{RaceID: 3, RaceType: "sprint", PlayerID: 1, Name: "A", Position: pos(2)},
	}
	got := Standings(results)
	if got[0].Name != "A" || got[0].Points != 25 || got[1].Points != 25 || got[1].Rank != 2 {
		t.Fatalf("classifica inattesa: %+v", got)
	}
}

func TestCompareTieBreak(t *testing.T) {
	a := Standing{Name: "A", Points: 50, positions: []int{2, 2, 3}}
	b := Standing{Name: "B", Points: 50, positions: []int{1, 5, 6}}
	if Compare(a, b) <= 0 {
		t.Error("a parità di punti chi ha più vittorie deve stare davanti")
	}

	c := Standing{Name: "C", Points: 50, positions: []int{1, 2}}
	d := Standing{Name: "D", Points: 50, positions: []int{1, 3}}
	if Compare(c, d) >= 0 {
		t.Error("a parità di vittorie contano i secondi posti")
	}

	e := Standing{Name: "elena", Points: 10}
	f := Standing{Name: "Fabio", Points: 10}
	if Compare(e, f) >= 0 {
		t.Error("a parità totale vale l'ordine alfabetico, senza distinguere maiuscole")
	}
}

func TestStandingsStats(t *testing.T) {
	results := []Result{
		{RaceID: 1, RaceType: "race", PlayerID: 1, Name: "A", Position: pos(1)},
		{RaceID: 2, RaceType: "race", PlayerID: 1, Name: "A", Position: pos(3)},
		{RaceID: 3, RaceType: "race", PlayerID: 1, Name: "A", DNF: true},
		{RaceID: 4, RaceType: "sprint", PlayerID: 1, Name: "A", Position: pos(1)},
	}
	s := Standings(results)[0]
	if s.Points != 25+15+8 || s.Wins != 1 || s.Podiums != 2 || s.Races != 3 || s.DNF != 1 || s.Rank != 1 {
		t.Fatalf("statistiche inattese: %+v", s)
	}
	if len(s.PerRace) != 4 || s.PerRace[4].Points != 8 {
		t.Fatalf("perRace inatteso: %+v", s.PerRace)
	}
}

func TestStandingsIgnoreSessionsWithoutPoints(t *testing.T) {
	results := []Result{
		{RaceID: 1, RaceType: "qualifying", PlayerID: 1, Name: "A", Position: pos(1)},
		{RaceID: 2, RaceType: "race", PlayerID: 1, Name: "A", Position: pos(2)},
		{RaceID: 3, RaceType: "practice", PlayerID: 2, Name: "B", Position: pos(1)},
	}
	standings := Standings(results)
	// B ha solo le prove libere: non compare in classifica.
	if len(standings) != 1 || standings[0].Points != 18 || standings[0].Races != 1 || len(standings[0].PerRace) != 1 {
		t.Fatalf("classifica %+v", standings)
	}
	if Counts("practice") || Counts("qualifying") || !Counts("race") || !Counts("sprint") {
		t.Error("Counts")
	}
}
