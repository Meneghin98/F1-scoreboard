package rotation

import (
	"slices"
	"testing"
)

func TestPickTracks(t *testing.T) {
	available := []int{11, 13, 5, 7, 10}
	// Monza e Suzuka sono già state corse: finché ce ne sono altre non tornano.
	raced := map[int]int{11: 1, 13: 2}
	for range 20 {
		got := PickTracks(available, raced, 3)
		slices.Sort(got)
		if !slices.Equal(got, []int{5, 7, 10}) {
			t.Fatalf("piste: %v, attese quelle mai corse", got)
		}
	}
	// Ne servono 4: tra le già corse viene Monza, corsa meno volte.
	for range 20 {
		got := PickTracks(available, raced, 4)
		slices.Sort(got)
		if !slices.Equal(got, []int{5, 7, 10, 11}) {
			t.Fatalf("piste: %v, attese le mai corse e Monza", got)
		}
	}
	// Più piste di quelle disponibili: tutte, una volta sola.
	got := PickTracks([]int{5, 5, 7}, nil, 10)
	slices.Sort(got)
	if !slices.Equal(got, []int{5, 7}) {
		t.Fatalf("piste: %v, attese 5 e 7", got)
	}
	if got := PickTracks(available, nil, 0); len(got) != 0 {
		t.Fatalf("nessuna pista richiesta: %v", got)
	}
}
