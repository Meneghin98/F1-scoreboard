package live

import (
	"bufio"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"maps"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Il replay si prepara rileggendo tutta la registrazione: su un Raspberry Pi
// Zero 2 W, per un'ora di gara, servono decine di secondi. Per non farli
// aspettare a chi lo apre, il server lo prepara appena finisce la sessione e lo
// salva accanto alla registrazione (<file>.replay): le volte dopo lo rilegge
// da lì in un attimo.
//
// Il file vale solo se l'ha scritto lo stesso eseguibile (dopo una nuova build
// il formato del replay potrebbe essere cambiato) e con gli stessi piloti alla
// guida (nomi e colori compaiono nel replay): altrimenti si rifà.

// replayExt è l'estensione del replay salvato, aggiunta al nome della registrazione.
const replayExt = ".replay"

// errStopping interrompe la preparazione di un replay quando il server si spegne.
var errStopping = errors.New("server in spegnimento")

// replayHeader identifica un replay salvato: lo si usa solo se è uguale a quello atteso.
type replayHeader struct {
	Executable string                   // eseguibile che l'ha scritto (vedi executableID)
	Recording  int64                    // dimensione della registrazione
	Players    map[int]scoreboardPlayer // piloti alla guida, per indice dell'auto
}

func (h replayHeader) matches(o replayHeader) bool {
	return h.Executable == o.Executable && h.Recording == o.Recording && maps.Equal(h.Players, o.Players)
}

// replayData è il replay come si salva: encoding/gob vuole i campi esportati.
type replayData struct {
	Duration time.Duration
	FrameAt  []time.Duration // istante di ogni fotogramma
	Frames   [][]byte        // fotogrammi compressi, come in memoria
	Pedals   []PedalSample
	// Il resto in JSON: gob non distingue una lista vuota da una mancante, che
	// arriverebbe al browser come null invece che come [].
	Meta []byte
}

type replayMeta struct {
	Laps     []ReplayLap   `json:"laps"`
	Events   []ReplayEvent `json:"events"`
	Analysis *Analysis     `json:"analysis"`
	TrackMap *TrackMap     `json:"trackMap"`
}

// executableID identifica l'eseguibile in esecuzione con percorso, data di
// modifica e dimensione: cambia a ogni build. Vuoto se non si riesce a
// saperlo: in quel caso i replay non si salvano.
var executableID = sync.OnceValue(func() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s|%d|%d", path, info.ModTime().UnixNano(), info.Size())
})

// saveReplay salva il replay in path. Prima si scrive un file temporaneo e poi
// lo si rinomina: al posto di un replay valido non resta mai uno scritto a metà.
func saveReplay(path string, h replayHeader, r *Replay) error {
	meta, err := json.Marshal(replayMeta{Laps: r.laps, Events: r.events, Analysis: r.analysis, TrackMap: r.trackMap})
	if err != nil {
		return err
	}
	d := replayData{Duration: r.Duration, Pedals: r.pedals, Meta: meta}
	for _, f := range r.frames {
		d.FrameAt = append(d.FrameAt, f.at)
		d.Frames = append(d.Frames, f.data)
	}

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	enc := gob.NewEncoder(w)
	err = enc.Encode(h)
	if err == nil {
		err = enc.Encode(d)
	}
	if err == nil {
		err = w.Flush()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// readReplay legge il replay salvato in path, se c'è e se l'intestazione è
// quella attesa: altrimenti restituisce nil e il replay va preparato da capo.
func readReplay(path string, want replayHeader) *Replay {
	f, err := os.Open(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Printf("telemetria: lettura del replay %s: %v", filepath.Base(path), err)
		}
		return nil
	}
	defer f.Close()
	dec := gob.NewDecoder(bufio.NewReader(f))
	var h replayHeader
	if err := dec.Decode(&h); err != nil || !h.matches(want) {
		return nil // file di un'altra versione o con altri piloti: si rifà
	}
	var d replayData
	var meta replayMeta
	if err := dec.Decode(&d); err != nil || len(d.FrameAt) != len(d.Frames) {
		log.Printf("telemetria: replay %s non valido: si rifà", filepath.Base(path))
		return nil
	}
	if err := json.Unmarshal(d.Meta, &meta); err != nil {
		log.Printf("telemetria: replay %s non valido: si rifà (%v)", filepath.Base(path), err)
		return nil
	}
	r := &Replay{Duration: d.Duration, pedals: d.Pedals, laps: meta.Laps, events: meta.Events, analysis: meta.Analysis,
		trackMap: meta.TrackMap, frames: make([]replayFrame, len(d.Frames))}
	for i := range d.Frames {
		r.frames[i] = replayFrame{at: d.FrameAt[i], data: d.Frames[i]}
	}
	return r
}

// prepareReplay prepara in background il replay della sessione appena finita
// e lo salva, così aprirlo dopo è immediato. Non lo tiene in memoria.
func (s *Service) prepareReplay(id int64) {
	s.background.Add(1)
	go func() {
		defer s.background.Done()
		if _, err := s.loadReplay(id); err != nil && !errors.Is(err, errStopping) {
			log.Printf("telemetria: preparazione del replay della sessione %d: %v", id, err)
		}
	}()
}

// dropReplay toglie il replay della sessione dalla memoria e dal disco:
// perché va rifatto (piloti cambiati) o perché la registrazione non c'è più.
func (s *Service) dropReplay(id int64, file string) {
	s.forgetReplay(id)
	if err := os.Remove(filepath.Join(s.opts.Dir, file+replayExt)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Printf("telemetria: eliminazione del replay di %s: %v", file, err)
	}
}
