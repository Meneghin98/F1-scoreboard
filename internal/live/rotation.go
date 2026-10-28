package live

import (
	"log"

	"f1-scoreboard/internal/rotation"
)

// rotationView restituisce la turnazione del torneo in corso, mostrata tra una
// sessione e l'altra. Si rilegge dal database solo dopo una modifica dei dati
// (vedi dataChangedLocked): presenti, sorteggio e gare salvate passano tutti da lì.
func (s *Service) rotationView() *rotation.State {
	if !s.rotationLoaded {
		st, err := rotation.Load(s.db)
		if err != nil {
			log.Printf("telemetria: turnazione: %v", err)
			return s.rotation
		}
		s.rotation, s.rotationLoaded = &st, true
	}
	return s.rotation
}

// rotationPlayers restituisce i piloti che la turnazione mette ai controller
// (1 e 2) nel prossimo Gran Premio, nil senza torneo in corso o senza almeno
// due presenti. Prove e qualifiche non fanno avanzare la turnazione, quindi
// valgono per tutto il weekend.
func (s *Service) rotationPlayers() map[int]int64 {
	st := s.rotationView()
	if st == nil || len(st.Next) == 0 {
		return nil
	}
	return map[int]int64{1: st.Next[0].P1, 2: st.Next[0].P2}
}
