// Package rotation decide chi prende i controller a ogni Gran Premio del
// torneo. In split-screen corrono due piloti alla volta: la turnazione fa sì
// che tra i presenti tutti corrano lo stesso numero di gare e incontrino tutti
// gli altri. Classifica e punti restano quelli del torneo (vedi scoring).
//
// La turnazione si può spegnere per un torneo (vedi State.Enabled): allora la
// pagina Live propone ai controller i piloti come senza torneo.
//
// Il programma (vedi plan.go) suggerisce su quali piste correre i prossimi
// Gran Premi, ognuna con la sua coppia se la turnazione è attiva.
//
// La turnazione non si salva: la prossima coppia si calcola ogni volta dai
// Gran Premi già corsi, così un pilota che arriva o va via, o una gara corsa
// fuori turno, non rompono nulla.
package rotation

import "slices"

// Pair sono i due piloti di un Gran Premio.
type Pair struct {
	P1 int64 `json:"p1"` // controller 1
	P2 int64 `json:"p2"` // controller 2
}

func (p Pair) has(id int64) bool { return p.P1 == id || p.P2 == id }

// Next restituisce le prossime n coppie tra i piloti presenti, dato lo storico
// dei Gran Premi già corsi in ordine (anche da piloti oggi assenti). Ogni
// coppia si sceglie come se le precedenti fossero già state corse. Con meno di
// due presenti non c'è nessuna coppia.
//
// Tra le coppie possibili vince, in ordine:
//  1. quella i cui piloti hanno corso meno gare in tutto;
//  2. quella che si è incontrata meno volte;
//  3. quella che non corre da più tempo;
//  4. a parità di tutto, quella con i piloti più avanti in order, l'ordine
//     sorteggiato all'inizio del torneo (vedi Draw); chi non c'è, perché
//     aggiunto dopo, viene in fondo per id. Senza sorteggio vale l'ordine degli id.
//
// A parità di tutto vuol dire anche la prima coppia del torneo, quando nessuno
// ha ancora corso: la decide il sorteggio.
//
// Chi è rimasto indietro, perché è arrivato dopo o ha saltato delle serate,
// recupera gradualmente: se ha appena corso salta il Gran Premio successivo,
// quindi ne corre uno sì e uno no finché non raggiunge gli altri. Le
// alternative valutate, nel caso si voglia cambiare:
//   - recupero pieno: basta togliere la regola, e chi è indietro corre di
//     fila finché non raggiunge gli altri (con 4 piloti a 3 gare, chi arriva
//     ne corre 3 di fila);
//   - nessun recupero: si contano le gare di ognuno solo da quando è arrivato,
//     così parte alla pari ma a fine torneo ha corso meno gare degli altri.
//
// Il controller 1 va a chi l'ha avuto meno volte, a parità al primo in order.
func Next(present []int64, history []Pair, order []int64, n int) []Pair {
	ids := slices.Clone(present)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if len(ids) < 2 {
		return nil
	}
	rank := func(id int64) int {
		if i := slices.Index(order, id); i >= 0 {
			return i
		}
		return len(order)
	}
	// A parità di rank (chi non è nel sorteggio) restano in ordine di id.
	slices.SortStableFunc(ids, func(a, b int64) int { return rank(a) - rank(b) })
	// Le coppie simulate non devono finire nello storico di chi chiama.
	history = slices.Clone(history)
	pairs := make([]Pair, 0, n)
	for range n {
		p := next(ids, history)
		pairs = append(pairs, p)
		history = append(history, p)
	}
	return pairs
}

// stats riassume lo storico di un pilota.
type stats struct {
	races       int
	controller1 int
	last        int // indice dell'ultimo Gran Premio corso, -1 se mai
}

func next(ids []int64, history []Pair) Pair {
	st := make(map[int64]*stats, len(ids))
	for _, id := range ids {
		st[id] = &stats{last: -1}
	}
	meetings := map[Pair]int{} // con P1 < P2, vedi ordered
	for i, p := range history {
		for _, id := range []int64{p.P1, p.P2} {
			if s := st[id]; s != nil {
				s.races++
				s.last = i
			}
		}
		if s := st[p.P1]; s != nil {
			s.controller1++
		}
		meetings[ordered(p.P1, p.P2)]++
	}

	most := 0
	for _, s := range st {
		most = max(most, s.races)
	}
	// Recupero graduale: chi è indietro e ha appena corso salta questo giro, se
	// al suo posto possono correre due piloti che hanno riposato. Altrimenti
	// (in tre) farebbe correre di nuovo chi non è indietro.
	justRaced := func(id int64) bool { return st[id].last == len(history)-1 }
	rested := 0
	for _, id := range ids {
		if !justRaced(id) {
			rested++
		}
	}
	resting := func(id int64) bool {
		return rested >= 2 && justRaced(id) && st[id].races < most
	}

	var candidates []Pair
	for i, a := range ids {
		for _, b := range ids[i+1:] {
			candidates = append(candidates, Pair{a, b})
		}
	}
	if allowed := slices.DeleteFunc(slices.Clone(candidates), func(p Pair) bool {
		return resting(p.P1) || resting(p.P2)
	}); len(allowed) > 0 {
		candidates = allowed
	}

	key := func(p Pair) []int {
		a, b := st[p.P1], st[p.P2]
		return []int{a.races + b.races, meetings[ordered(p.P1, p.P2)], a.last + b.last}
	}
	best := slices.MinFunc(candidates, func(x, y Pair) int { // a parità tiene la prima, cioè la più avanti in order
		return slices.Compare(key(x), key(y))
	})
	if st[best.P2].controller1 < st[best.P1].controller1 {
		best.P1, best.P2 = best.P2, best.P1
	}
	return best
}

func ordered(a, b int64) Pair {
	if a > b {
		a, b = b, a
	}
	return Pair{a, b}
}
