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
		if got := (Rules{}).PointsFor(tt.raceType, 20, tt.position, tt.dnf); got != tt.want {
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
	got := Standings(results, Rules{})
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
	s := Standings(results, Rules{})[0]
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
	standings := Standings(results, Rules{})
	// B ha solo le prove libere: non compare in classifica.
	if len(standings) != 1 || standings[0].Points != 18 || standings[0].Races != 1 || len(standings[0].PerRace) != 1 {
		t.Fatalf("classifica %+v", standings)
	}
	var rules Rules
	if rules.Counts("practice") || rules.Counts("qualifying") || !rules.Counts("race") || !rules.Counts("sprint") {
		t.Error("Counts")
	}
	// Con i punti per la pole le qualifiche entrano in classifica, le prove libere mai.
	if rules := (Rules{PolePoints: 1}); !rules.Counts("qualifying") || rules.Counts("practice") {
		t.Error("Counts con i punti per la pole")
	}
}

// Le qualifiche vinte contano come pole, i giri veloci solo nei Gran Premi.
func TestPolesAndFastestLaps(t *testing.T) {
	p := func(n int) *int { return &n }
	st := Standings([]Result{
		{RaceID: 1, RaceType: "qualifying", PlayerID: 1, Name: "A", Position: p(1)},
		{RaceID: 1, RaceType: "qualifying", PlayerID: 2, Name: "B", Position: p(2)},
		{RaceID: 2, RaceType: "race", PlayerID: 1, Name: "A", Position: p(2)},
		{RaceID: 2, RaceType: "race", PlayerID: 2, Name: "B", Position: p(1), FastestLap: true},
		{RaceID: 3, RaceType: "sprint", PlayerID: 1, Name: "A", Position: p(1), FastestLap: true},
	}, Rules{})
	byName := map[string]Standing{}
	for _, s := range st {
		byName[s.Name] = s
	}
	if a, b := byName["A"], byName["B"]; a.Poles != 1 || b.Poles != 0 || a.Fastest != 0 || b.Fastest != 1 {
		t.Fatalf("A %+v, B %+v", a, b)
	}
	if a := byName["A"]; a.Points != 18+8 || len(a.PerRace) != 2 {
		t.Fatalf("punti di A %+v", a)
	}
}

// In modalità "players" le posizioni si ricalcolano tra i soli piloti: le auto
// della IA davanti non tolgono punti. Vittorie, podi e countback usano le stesse posizioni.
func TestPlayersMode(t *testing.T) {
	results := []Result{
		{RaceID: 1, RaceType: "race", PlayerID: 1, Name: "A", Position: pos(11)},
		{RaceID: 1, RaceType: "race", PlayerID: 2, Name: "B", Position: pos(3)},
		{RaceID: 1, RaceType: "race", PlayerID: 3, Name: "C", DNF: true},
		{RaceID: 1, RaceType: "race", PlayerID: 4, Name: "D", Position: pos(14)},
		{RaceID: 2, RaceType: "sprint", PlayerID: 1, Name: "A", Position: pos(9)},
		{RaceID: 2, RaceType: "sprint", PlayerID: 2, Name: "B", Position: pos(12)},
	}
	points := func(st []Standing) map[string]int {
		m := map[string]int{}
		for _, s := range st {
			m[s.Name] = s.Points
		}
		return m
	}

	absolute := points(Standings(results, Rules{Mode: ModeAbsolute}))
	if absolute["A"] != 0 || absolute["B"] != 15 || absolute["C"] != 0 || absolute["D"] != 0 {
		t.Errorf("modalità assoluta: %v", absolute)
	}

	st := Standings(results, Rules{Mode: ModePlayers})
	// GP: B 1° (25), A 2° (18), D 3° (15), C ritirato. Sprint: A 1° (8), B 2° (7).
	if got := points(st); got["A"] != 18+8 || got["B"] != 25+7 || got["C"] != 0 || got["D"] != 15 {
		t.Fatalf("modalità piloti: %v", got)
	}
	b := st[0]
	if b.Name != "B" || b.Wins != 1 || b.Podiums != 1 || *b.PerRace[1].Position != 1 || *b.PerRace[2].Position != 2 {
		t.Errorf("B %+v", b)
	}
	if c := st[3]; c.Name != "C" || c.DNF != 1 || c.PerRace[1].Position != nil || !c.PerRace[1].DNF {
		t.Errorf("C %+v", c)
	}
}

// I punti per la pole vanno a chi parte primo in qualifica, IA compresa, anche
// in modalità "players": il primo dei piloti dietro alla IA non li prende.
func TestPolePoints(t *testing.T) {
	results := []Result{
		{RaceID: 1, RaceType: "qualifying", PlayerID: 1, Name: "A", Position: pos(1)},
		{RaceID: 1, RaceType: "qualifying", PlayerID: 2, Name: "B", Position: pos(3)},
		{RaceID: 2, RaceType: "qualifying", PlayerID: 1, Name: "A", Position: pos(4)},
		{RaceID: 2, RaceType: "qualifying", PlayerID: 2, Name: "B", Position: pos(6)},
		{RaceID: 3, RaceType: "race", PlayerID: 1, Name: "A", Position: pos(2)},
		{RaceID: 3, RaceType: "race", PlayerID: 2, Name: "B", Position: pos(1)},
	}
	for _, mode := range []string{ModeAbsolute, ModePlayers} {
		st := Standings(results, Rules{Mode: mode, PolePoints: 3})
		a := st[1]
		if a.Name != "A" || a.Points != 3+18 || a.Poles != 1 || a.PerRace[1].Points != 3 || a.PerRace[1].Bonus != 3 || a.Races != 1 {
			t.Fatalf("%s: A %+v", mode, a)
		}
		if a.PerRace[2].Points != 0 || a.PerRace[2].Bonus != 0 {
			t.Errorf("%s: pole della IA, A %+v", mode, a.PerRace[2])
		}
		if b := st[0]; b.Points != 25 || b.Poles != 0 || b.PerRace[1].Points != 0 || b.PerRace[2].Points != 0 {
			t.Fatalf("%s: B %+v", mode, b)
		}
	}
	// Senza punti per la pole le qualifiche non entrano in classifica, ma la pole si conta.
	st := Standings(results, Rules{Mode: ModePlayers})
	if a := st[1]; a.Poles != 1 || len(a.PerRace) != 1 || a.Points != 18 {
		t.Fatalf("senza punti per la pole: %+v", a)
	}
}

// Il punto per il giro veloce vale solo nei Gran Premi e solo per chi arriva.
func TestFastestLapPoints(t *testing.T) {
	rules := Rules{FastestLapPoints: 1}
	scores := ScoreRace([]Result{
		{RaceType: "race", Position: pos(1)},
		{RaceType: "race", Position: pos(2), FastestLap: true},
	}, rules)
	if scores[0].Points != 25 || scores[1].Points != 19 || scores[1].Bonus != 1 || !scores[1].FastestLap {
		t.Errorf("Gran Premio: %+v", scores)
	}
	scores = ScoreRace([]Result{{RaceType: "race", DNF: true, FastestLap: true}}, rules)
	if scores[0].Points != 0 || scores[0].Bonus != 0 || !scores[0].FastestLap {
		t.Errorf("ritirato: %+v", scores)
	}
	scores = ScoreRace([]Result{{RaceType: "sprint", Position: pos(1), FastestLap: true}}, rules)
	if scores[0].Points != 8 {
		t.Errorf("sprint: %+v", scores)
	}
}

// Anche in modalità "players" il giro veloce è quello della gara, IA compresa:
// se l'ha fatto la IA il punto non va a nessuno.
func TestFastestLapPlayersMode(t *testing.T) {
	rules := Rules{Mode: ModePlayers, FastestLapPoints: 2}
	scores := ScoreRace([]Result{
		{RaceType: "race", Position: pos(5)},
		{RaceType: "race", Position: pos(7)},
	}, rules)
	if scores[0].Points != 25 || scores[1].Points != 18 || scores[0].FastestLap || scores[1].FastestLap {
		t.Errorf("giro veloce della IA: %+v", scores)
	}
	scores = ScoreRace([]Result{
		{RaceType: "race", Position: pos(5)},
		{RaceType: "race", Position: pos(7), FastestLap: true},
	}, rules)
	if scores[0].Points != 25 || scores[1].Points != 18+2 || !scores[1].FastestLap || scores[1].Bonus != 2 {
		t.Errorf("giro veloce di un pilota: %+v", scores)
	}
	// Il più veloce si è ritirato: il punto non va a nessuno.
	scores = ScoreRace([]Result{
		{RaceType: "race", Position: pos(1)},
		{RaceType: "race", DNF: true, FastestLap: true},
	}, rules)
	if scores[0].Points != 25 || scores[1].Points != 0 || !scores[1].FastestLap {
		t.Errorf("più veloce ritirato: %+v", scores)
	}
	// In classifica si contano solo i giri veloci veri, anche se non danno punti.
	st := Standings([]Result{
		{RaceID: 1, RaceType: "race", PlayerID: 1, Name: "A", Position: pos(3)},
		{RaceID: 1, RaceType: "race", PlayerID: 2, Name: "B", Position: pos(8)},
		{RaceID: 2, RaceType: "race", PlayerID: 1, Name: "A", Position: pos(4)},
		{RaceID: 2, RaceType: "race", PlayerID: 2, Name: "B", DNF: true, FastestLap: true},
	}, Rules{Mode: ModePlayers})
	if st[0].Name != "A" || st[0].Fastest != 0 || st[1].Fastest != 1 || st[1].PerRace[2].Bonus != 0 {
		t.Errorf("classifica %+v", st)
	}
}

// In modalità ModePlayers si possono scegliere i punti per posizione; in
// modalità ModeAbsolute vale sempre la tabella F1.
func TestCustomPointsTable(t *testing.T) {
	results := []Result{
		{RaceID: 1, RaceType: "race", PlayerID: 1, Name: "A", Position: pos(5)},
		{RaceID: 1, RaceType: "race", PlayerID: 2, Name: "B", Position: pos(8)},
		{RaceID: 1, RaceType: "race", PlayerID: 3, Name: "C", Position: pos(9)},
		{RaceID: 2, RaceType: "sprint", PlayerID: 2, Name: "B", Position: pos(1)},
	}
	rules := Rules{Mode: ModePlayers, Preset: PresetCustom, RacePoints: PointsTable{10, 6}, SprintPoints: PointsTable{3}}
	points := map[string]int{}
	for _, s := range Standings(results, rules) {
		points[s.Name] = s.Points
	}
	if points["A"] != 10 || points["B"] != 6+3 || points["C"] != 0 {
		t.Errorf("punti con la tabella scelta %v", points)
	}

	rules.Mode = ModeAbsolute
	if got := rules.PointsFor("race", 20, pos(1), false); got != 25 {
		t.Errorf("in modalità assoluta il primo prende %d punti, attesi 25", got)
	}
}

func TestPointsTableDatabase(t *testing.T) {
	var p PointsTable
	if err := p.Scan("10,6,3"); err != nil || len(p) != 3 || p[2] != 3 {
		t.Fatalf("Scan = %v, %v", p, err)
	}
	if v, _ := p.Value(); v != "10,6,3" {
		t.Errorf("Value = %v", v)
	}
	if err := p.Scan(""); err != nil || p != nil {
		t.Errorf("il testo vuoto deve dare nil: %v, %v", p, err)
	}
	if v, _ := PointsTable(nil).Value(); v != "" {
		t.Errorf("Value di nil = %q", v)
	}
}

// Le tabelle già pronte: Rank Down dipende da quanti piloti corrono la gara,
// con Push & Pay anche i ritirati perdono un punto.
func TestPresets(t *testing.T) {
	results := []Result{
		{RaceID: 1, RaceType: "race", PlayerID: 1, Name: "A", Position: pos(3)},
		{RaceID: 1, RaceType: "race", PlayerID: 2, Name: "B", Position: pos(7)},
		{RaceID: 1, RaceType: "race", PlayerID: 3, Name: "C", DNF: true},
		{RaceID: 2, RaceType: "sprint", PlayerID: 2, Name: "B", Position: pos(2)},
		{RaceID: 2, RaceType: "sprint", PlayerID: 1, Name: "A", Position: pos(4)},
	}
	tests := map[string]map[string]int{
		PresetUltimateChampion: {"A": 1, "B": 0, "C": 0},
		PresetF1:               {"A": 25 + 7, "B": 18 + 8, "C": 0},
		PresetRankDown:         {"A": 3 + 1, "B": 2 + 2, "C": 0},
		PresetAllOrNothing:     {"A": 25, "B": 8, "C": 0},
		PresetPushAndPay:       {"A": 2 - 1, "B": -1 + 2, "C": -1},
		"":                     {"A": 25 + 7, "B": 18 + 8, "C": 0},
	}
	for preset, want := range tests {
		got := map[string]int{}
		for _, s := range Standings(results, Rules{Mode: ModePlayers, Preset: preset}) {
			got[s.Name] = s.Points
		}
		for name, points := range want {
			if got[name] != points {
				t.Errorf("%q: %s ha %d punti, attesi %d", preset, name, got[name], points)
			}
		}
	}
}

// Ultimate Champion: un punto al primo tra i piloti, uno in più a chi vince la
// gara davanti alla IA; le sprint danno solo quello. Chi si ritira lascia il
// punto all'altro, anche se arriva dietro a tutta la IA.
func TestUltimateChampion(t *testing.T) {
	rules := Rules{Mode: ModePlayers, Preset: PresetUltimateChampion, FastestLapPoints: 1}
	tests := []struct {
		name    string
		results []Result
		want    []int
		win     []int
	}{
		{"primo tra i piloti", []Result{
			{RaceType: "race", Position: pos(3)}, {RaceType: "race", Position: pos(5)},
		}, []int{1, 0}, []int{0, 0}},
		{"vittoria davanti alla IA", []Result{
			{RaceType: "race", Position: pos(4)}, {RaceType: "race", Position: pos(1)},
		}, []int{0, 2}, []int{0, 1}},
		{"ritirato", []Result{
			{RaceType: "race", DNF: true}, {RaceType: "race", Position: pos(15)},
		}, []int{0, 1}, []int{0, 0}},
		{"vittoria con il giro veloce", []Result{
			{RaceType: "race", Position: pos(1), FastestLap: true}, {RaceType: "race", Position: pos(2)},
		}, []int{3, 0}, []int{1, 0}},
		{"sprint", []Result{
			{RaceType: "sprint", Position: pos(2)}, {RaceType: "sprint", Position: pos(6)},
		}, []int{0, 0}, []int{0, 0}},
		{"sprint vinta davanti alla IA", []Result{
			{RaceType: "sprint", Position: pos(1)}, {RaceType: "sprint", Position: pos(6)},
		}, []int{1, 0}, []int{1, 0}},
		{"tre piloti", []Result{
			{RaceType: "race", Position: pos(8)}, {RaceType: "race", Position: pos(2)}, {RaceType: "race", Position: pos(5)},
		}, []int{0, 1, 0}, []int{0, 0, 0}},
	}
	for _, tt := range tests {
		for i, sc := range ScoreRace(tt.results, rules) {
			if sc.Points != tt.want[i] || sc.WinBonus != tt.win[i] {
				t.Errorf("%s: pilota %d con %d punti (vittoria +%d), attesi %d (+%d)", tt.name, i+1, sc.Points, sc.WinBonus, tt.want[i], tt.win[i])
			}
		}
	}

	// In classifica le sprint contano, e il punto della vittoria finisce nella gara.
	st := Standings([]Result{
		{RaceID: 1, RaceType: "sprint", PlayerID: 1, Name: "A", Position: pos(1)},
		{RaceID: 1, RaceType: "sprint", PlayerID: 2, Name: "B", Position: pos(3)},
		{RaceID: 2, RaceType: "race", PlayerID: 1, Name: "A", Position: pos(4)},
		{RaceID: 2, RaceType: "race", PlayerID: 2, Name: "B", Position: pos(1)},
	}, rules)
	if st[0].Name != "B" || st[0].Points != 2 || st[0].PerRace[2].WinBonus != 1 || st[1].Points != 1 || st[1].PerRace[1].WinBonus != 1 {
		t.Errorf("classifica %+v", st)
	}
}
