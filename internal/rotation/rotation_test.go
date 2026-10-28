package rotation

import (
	"slices"
	"testing"
)

const (
	A int64 = iota + 1
	B
	C
	D
	E
)

// count restituisce le gare corse da ogni pilota.
func count(history []Pair) map[int64]int {
	races := map[int64]int{}
	for _, p := range history {
		races[p.P1]++
		races[p.P2]++
	}
	return races
}

func TestNextFourPlayers(t *testing.T) {
	got := Next([]int64{D, B, C, A}, nil, 6)
	want := []Pair{{A, B}, {C, D}, {A, C}, {B, D}, {A, D}, {B, C}}
	if len(got) != len(want) {
		t.Fatalf("coppie: %v, attese %v", got, want)
	}
	for i := range want {
		if ordered(got[i].P1, got[i].P2) != want[i] {
			t.Fatalf("coppie: %v, attese (a meno dei controller) %v", got, want)
		}
	}
	// Dopo un giro completo ognuno ha corso 3 gare; i 6 controller 1 vanno 1 o 2 a testa.
	ones := map[int64]int{}
	for _, p := range got {
		ones[p.P1]++
	}
	for _, id := range []int64{A, B, C, D} {
		if count(got)[id] != 3 {
			t.Errorf("pilota %d: %d gare, attese 3", id, count(got)[id])
		}
		if ones[id] < 1 || ones[id] > 2 {
			t.Errorf("pilota %d: %d volte col controller 1, attese 1 o 2", id, ones[id])
		}
	}
}

func TestNextContinuesFromHistory(t *testing.T) {
	all := Next([]int64{A, B, C, D}, nil, 12)
	rest := Next([]int64{A, B, C, D}, all[:5], 7)
	if !slices.Equal(all[5:], rest) {
		t.Fatalf("partendo dallo storico: %v, attese %v", rest, all[5:])
	}
	// Due giri completi: ogni coppia si incontra due volte.
	meetings := map[Pair]int{}
	for _, p := range all {
		meetings[ordered(p.P1, p.P2)]++
	}
	for p, n := range meetings {
		if n != 2 {
			t.Errorf("coppia %v: %d incontri, attesi 2", p, n)
		}
	}
}

func TestNextFivePlayers(t *testing.T) {
	ids := []int64{A, B, C, D, E}
	got := Next(ids, nil, 10)
	// In ogni momento nessuno ha più di una gara di vantaggio sugli altri.
	for i := range got {
		races := count(got[:i+1])
		lo, hi := races[A], races[A]
		for _, id := range ids {
			lo, hi = min(lo, races[id]), max(hi, races[id])
		}
		if hi-lo > 1 {
			t.Fatalf("dopo %d gare %v: gare per pilota %v", i+1, got[:i+1], races)
		}
	}
	for _, id := range ids {
		if count(got)[id] != 4 {
			t.Errorf("pilota %d: %d gare dopo 10, attese 4 (%v)", id, count(got)[id], got)
		}
	}
}

func TestNextLateArrivalCatchesUpGradually(t *testing.T) {
	history := Next([]int64{A, B, C, D}, nil, 6) // 3 gare a testa
	got := Next([]int64{A, B, C, D, E}, history, 12)
	// Finché è indietro E corre un Gran Premio sì e uno no, a partire dal primo.
	for i, p := range got {
		races := count(append(slices.Clone(history), got[:i]...))
		behind := races[E] < max(races[A], races[B], races[C], races[D])
		if !behind {
			break
		}
		if want := i%2 == 0; p.has(E) != want {
			t.Fatalf("gara %d: E corre = %v, atteso %v (%v)", i+1, p.has(E), want, got)
		}
	}
	if races := count(append(history, got...)); races[E] < 3 {
		t.Errorf("dopo 12 gare E ne ha corse solo %d (%v)", races[E], got)
	}
}

func TestNextLateArrivalWithFourPresent(t *testing.T) {
	// B stasera non c'è: E arriva al posto suo e recupera anche in quattro.
	history := Next([]int64{A, B, C, D}, nil, 6)
	got := Next([]int64{A, C, D, E}, history, 4)
	for i, p := range got {
		if want := i%2 == 0; p.has(E) != want {
			t.Fatalf("gara %d: E corre = %v, atteso %v (%v)", i+1, p.has(E), want, got)
		}
	}
}

func TestNextFewPlayers(t *testing.T) {
	if got := Next([]int64{A}, nil, 3); got != nil {
		t.Errorf("con un pilota: %v, attese nessuna coppia", got)
	}
	if got := Next(nil, []Pair{{A, B}}, 3); got != nil {
		t.Errorf("senza presenti: %v, attese nessuna coppia", got)
	}
	for _, p := range Next([]int64{B, A}, nil, 4) {
		if ordered(p.P1, p.P2) != (Pair{A, B}) {
			t.Fatalf("con due piloti la coppia è sempre la stessa, non %v", p)
		}
	}
}

func TestNextThreePlayers(t *testing.T) {
	// In tre qualcuno corre per forza di fila, ma dopo 3 gare tutti ne hanno corse 2.
	got := Next([]int64{A, B, C}, nil, 3)
	for _, id := range []int64{A, B, C} {
		if count(got)[id] != 2 {
			t.Errorf("pilota %d: %d gare, attese 2 (%v)", id, count(got)[id], got)
		}
	}
}

func TestNextIgnoresAbsentInHistory(t *testing.T) {
	// E ha corso con A, ma stasera non c'è: le gare di A contano lo stesso.
	got := Next([]int64{A, B, C}, []Pair{{A, E}}, 1)
	if got[0].has(A) {
		t.Errorf("A ha già corso: la prima coppia dovrebbe essere B-C, non %v", got[0])
	}
}

func TestNextDoesNotModifyHistory(t *testing.T) {
	history := make([]Pair, 1, 10)
	history[0] = Pair{A, B}
	Next([]int64{A, B, C, D}, history, 5)
	if full := history[:cap(history)]; full[1] != (Pair{}) {
		t.Errorf("Next ha scritto nello storico di chi chiama: %v", full)
	}
}
