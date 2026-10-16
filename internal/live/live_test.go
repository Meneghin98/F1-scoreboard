package live

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"f1-scoreboard/internal/recording"
	"f1-scoreboard/internal/store"
	"f1-scoreboard/internal/telemetry"
)

// loadRecording legge i pacchetti della registrazione reale in testdata.
func loadRecording(t *testing.T) []recording.Packet {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "recordings", "*.f1rec.gz"))
	if len(files) == 0 {
		t.Skip("nessuna registrazione in testdata/recordings")
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if bytes.HasPrefix(data, []byte("version https://git-lfs")) {
		t.Skip("registrazione non scaricata da Git LFS: esegui git lfs pull")
	}
	r, err := recording.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var packets []recording.Packet
	for {
		p, err := r.Next()
		if errors.Is(err, io.EOF) {
			return packets
		}
		if err != nil {
			t.Fatal(err)
		}
		packets = append(packets, p)
	}
}

func TestTrackerOnRecording(t *testing.T) {
	var tr Tracker
	for _, p := range loadRecording(t) {
		if h, err := telemetry.ParseHeader(p.Data); err == nil && h.SessionUID != 0 {
			tr.Handle(h, p.Data)
		}
	}

	snap := tr.Snapshot("live")
	if snap.Session == nil || snap.Session.Track != "Monza" || snap.Session.Type != "Gara" || !snap.Session.IsRace {
		t.Fatalf("sessione %+v", snap.Session)
	}
	if !snap.Finished || snap.Session.TotalLaps != 5 {
		t.Errorf("finita %v, giri %d", snap.Finished, snap.Session.TotalLaps)
	}
	if len(snap.Drivers) != 20 {
		t.Fatalf("%d piloti, attesi 20", len(snap.Drivers))
	}
	players := 0
	for i, d := range snap.Drivers {
		if int(d.Position) != i+1 {
			t.Errorf("piloti non ordinati: posizione %d all'indice %d", d.Position, i)
		}
		if d.Name == "" || d.Team == "" || d.Tyre == "" {
			t.Errorf("dati mancanti: %+v", d)
		}
		if d.IsPlayer {
			players++
			if d.Name != "HAMILTON" || d.BestLapMS != 86_432 {
				t.Errorf("giocatore %+v", d)
			}
		}
	}
	if players != 1 || snap.Drivers[0].Name != "NORRIS" {
		t.Errorf("%d giocatori, primo %s", players, snap.Drivers[0].Name)
	}
}

func newTestService(t *testing.T, record bool) (*Service, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	sessions := filepath.Join(dir, "sessions")
	s := NewService(db, Options{Dir: sessions, Record: record})
	// Chiude l'eventuale registrazione aperta prima che il test cancelli i file
	// (su Windows un file aperto non si può eliminare).
	t.Cleanup(func() {
		s.mu.Lock()
		s.endSession(time.Now())
		s.mu.Unlock()
	})
	return s, sessions
}

// Passa la registrazione al servizio come se arrivasse dal gioco, con i tempi originali.
func TestServiceRecordsSessions(t *testing.T) {
	packets := loadRecording(t)
	s, dir := newTestService(t, true)
	base := time.Date(2026, 10, 16, 13, 31, 38, 0, time.UTC)

	for _, p := range packets {
		s.Handle(base.Add(p.At), p.Data)
	}
	// Gli ultimi pacchetti della registrazione sono di menù (senza sessione):
	// la sessione finisce con l'ultimo pacchetto che le appartiene.
	end := s.lastPacket

	// A sessione in corso lo stato è "live".
	var snap Snapshot
	json.Unmarshal(s.snapshotJSON(end), &snap)
	if snap.Status != "live" || snap.Session == nil || snap.Session.Track != "Monza" {
		t.Fatalf("stato durante la sessione: %+v", snap)
	}
	live, _ := s.Sessions()
	if len(live) == 0 || !live[0].Recording {
		t.Fatalf("l'ultima sessione dovrebbe risultare in registrazione: %+v", live)
	}

	// Dopo un minuto senza pacchetti la sessione viene chiusa.
	s.tick(end.Add(sessionTimeout + time.Second))
	json.Unmarshal(s.snapshotJSON(end.Add(2*sessionTimeout)), &snap)
	if snap.Status != "waiting" || snap.Session != nil {
		t.Fatalf("stato dopo la fine: %+v", snap)
	}

	records, err := s.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 5 {
		t.Fatalf("%d sessioni, attese 5", len(records))
	}
	total := 0
	for _, r := range records {
		if r.Track != "Monza" || r.Type != "Gara" || !r.IsRace || r.EndedAt == nil || r.Recording || !r.Downloadable {
			t.Errorf("sessione %+v", r)
		}
		// Il file contiene esattamente i pacchetti della sessione.
		path, name, err := s.SessionFile(r.ID)
		if err != nil {
			t.Fatal(err)
		}
		if n := countPackets(t, path); n != r.Packets {
			t.Errorf("sessione %d: %d pacchetti nel file, %d nel database", r.ID, n, r.Packets)
		}
		if filepath.Ext(name) != ".gz" || !bytes.Contains([]byte(name), []byte("_monza_gara")) {
			t.Errorf("nome per il download %q", name)
		}
		total += r.Packets
	}
	if !records[0].HasClassification || records[1].HasClassification {
		t.Error("solo l'ultima sessione ha la classifica finale")
	}
	if files, _ := os.ReadDir(dir); len(files) != 5 {
		t.Errorf("%d file registrati", len(files))
	}
	t.Logf("%d pacchetti di sessione su %d totali (gli altri sono di menù e caricamenti)", total, len(packets))
}

func countPackets(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := recording.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for {
		if _, err := r.Next(); err != nil {
			if !errors.Is(err, io.EOF) {
				t.Fatal(err)
			}
			return n
		}
		n++
	}
}

func TestServiceWithoutRecording(t *testing.T) {
	packets := loadRecording(t)
	s, dir := newTestService(t, false)
	for _, p := range packets[:5000] {
		s.Handle(time.Now(), p.Data)
	}
	records, _ := s.Sessions()
	if len(records) == 0 || records[0].Downloadable {
		t.Fatalf("sessioni %+v", records)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("senza registrazione non devono essere creati file")
	}
}

func TestSubscribe(t *testing.T) {
	s, _ := newTestService(t, false)
	updates, unsubscribe := s.Subscribe()
	defer unsubscribe()

	var snap Snapshot
	json.Unmarshal(<-updates, &snap)
	if snap.Status != "waiting" {
		t.Fatalf("primo stato %+v", snap)
	}

	// Un pacchetto di sessione cambia lo stato: il tick lo invia agli iscritti.
	data := telemetryPacket(t, telemetry.PacketEvent, 42, "SSTA")
	s.Handle(time.Now(), data)
	s.tick(time.Now())
	select {
	case msg := <-updates:
		json.Unmarshal(msg, &snap)
		if snap.Status != "live" || snap.Session == nil || snap.Session.UID != "000000000000002a" {
			t.Fatalf("stato aggiornato %+v", snap)
		}
	default:
		t.Fatal("nessun aggiornamento dopo il nuovo pacchetto")
	}

	// Se nulla cambia non viene inviato nulla.
	s.tick(time.Now())
	select {
	case msg := <-updates:
		t.Fatalf("aggiornamento inatteso: %s", msg)
	default:
	}
}

func telemetryPacket(t *testing.T, id telemetry.PacketID, uid uint64, payload string) []byte {
	t.Helper()
	data := make([]byte, telemetry.HeaderSize, 45)
	data[0], data[1] = 0xE9, 0x07 // 2025
	data[6] = byte(id)
	for i := range 8 {
		data[7+i] = byte(uid >> (8 * i))
	}
	data = append(data, payload...)
	return append(data, make([]byte, 45-len(data))...)
}

func TestListener(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	received := make(chan string, 10)
	ready := make(chan net.Addr, 1)
	l := &Listener{
		Addr:   "127.0.0.1:0", // porta libera scelta dal sistema
		Handle: func(_ time.Time, data []byte) { received <- string(data) },
		Ready:  func(addr net.Addr) { ready <- addr },
	}
	stopped := make(chan struct{})
	go func() {
		l.Run(ctx)
		close(stopped)
	}()

	addr := <-ready
	conn, err := net.Dial("udp", addr.String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, msg := range []string{"uno", "due"} {
		conn.Write([]byte(msg))
		select {
		case got := <-received:
			if got != msg {
				t.Fatalf("ricevuto %q, atteso %q", got, msg)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("pacchetto non ricevuto")
		}
	}

	cancel()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("il listener non si ferma")
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{"Sakhir (Bahrain)": "sakhir-bahrain", "Prove libere 1": "prove-libere-1", "Gara": "gara"} {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, atteso %q", in, got, want)
		}
	}
}

func TestTrackMapOnRecording(t *testing.T) {
	var tr Tracker
	for _, p := range loadRecording(t) {
		if h, err := telemetry.ParseHeader(p.Data); err == nil && h.SessionUID != 0 {
			tr.Handle(h, p.Data)
		}
	}
	m := tr.TrackMap()
	if m == nil {
		t.Fatal("nessuna mappa")
	}
	// Con 20 auto per 5 giri ogni tratto di Monza deve essere stato visto.
	want := (m.Length + outlineStep - 1) / outlineStep
	if len(m.Points) < want*95/100 {
		t.Errorf("%d punti su %d", len(m.Points), want)
	}
	// Punti consecutivi distano circa outlineStep metri: un salto grande vorrebbe
	// dire posizioni associate al tratto sbagliato.
	for i := 1; i < len(m.Points); i++ {
		a, b := m.Points[i-1], m.Points[i]
		if d := math.Hypot(float64(b.X-a.X), float64(b.Z-a.Z)); d > 4*outlineStep {
			t.Errorf("salto di %.0f m al punto %d", d, i)
		}
	}
	snap := tr.Snapshot("live")
	if snap.Session.MapPoints != len(m.Points) {
		t.Errorf("mapPoints %d, punti %d", snap.Session.MapPoints, len(m.Points))
	}
	// Monza si percorre in senso orario. Disegnando X verso destra e Z verso il
	// basso (come fa il frontend) l'area con segno deve essere positiva:
	// se fosse negativa la mappa risulterebbe specchiata.
	var area float64
	for i, a := range m.Points {
		b := m.Points[(i+1)%len(m.Points)]
		area += float64(a.X*b.Z - b.X*a.Z)
	}
	if area <= 0 {
		t.Errorf("mappa specchiata: area con segno %.0f", area/2)
	}
}
