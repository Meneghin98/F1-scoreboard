package update

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"f1-scoreboard/internal/version"
)

const (
	// Repository GitHub da cui arrivano le release.
	repo = "Meneghin98/F1-scoreboard"
	// Il primo controllo si fa poco dopo l'avvio, poi ogni checkEvery. GitHub
	// permette 60 richieste all'ora senza autenticazione: ne usiamo pochissime.
	firstCheck = 30 * time.Second
	checkEvery = 6 * time.Hour
	// Ogni quanto si guarda se è il momento di installare una versione scaricata.
	installEvery = 5 * time.Minute
	// Limiti alle dimensioni dei file scaricati, contro risposte sbagliate.
	maxBinarySize = 200 << 20
	maxSmallSize  = 1 << 20
)

// Suffissi dei file accanto all'eseguibile:
//
//	.download     download in corso, non ancora verificato
//	.new          versione nuova verificata, da installare
//	.new.version  numero della versione in .new
//	.old          versione precedente, tenuta dopo l'installazione per tornare indietro a mano
const (
	downloadSuffix = ".download"
	newSuffix      = ".new"
	versionSuffix  = ".new.version"
	oldSuffix      = ".old"
)

// Status è lo stato degli aggiornamenti, mostrato nella pagina Sistema.
type Status struct {
	Enabled   bool   `json:"enabled"`   // falso nelle build di sviluppo o con UPDATE=false
	Current   string `json:"current"`   // versione in esecuzione, vuota nelle build di sviluppo
	Latest    string `json:"latest"`    // ultima release su GitHub, vuota se non ancora controllato
	Staged    string `json:"staged"`    // versione scaricata e verificata, da installare
	CheckedAt string `json:"checkedAt"` // ultimo controllo riuscito, UTC come datetime('now') di SQLite
	Error     string `json:"error"`     // errore dell'ultimo controllo o download, vuoto se è andato bene
	// Versione nuova trovata e non ancora scaricata: dopo un controllo manuale si
	// scarica con StartDownload, dopo la conferma nella pagina Sistema.
	Available  string `json:"available"`
	Phase      string `json:"phase"`      // operazione in corso (Phase...), vuota se nessuna
	Size       int64  `json:"size"`       // byte dell'eseguibile da scaricare, 0 se non si sa
	Downloaded int64  `json:"downloaded"` // byte già scaricati durante un download
	// Ora (locale) in cui il server si riavvia per installare una versione scaricata.
	InstallHour int `json:"installHour"`
}

// Valori di Status.Phase.
const (
	PhaseChecking    = "checking"    // lettura dell'ultima release su GitHub
	PhaseDownloading = "downloading" // download e verifica della versione nuova
)

// pending è una versione nuova trovata da un controllo, con i file da scaricare.
type pending struct {
	number string
	tag    string
	urls   map[string]string // nome del file → URL
}

// Options configura gli aggiornamenti.
type Options struct {
	Exe     string // eseguibile da aggiornare
	Enabled bool   // falso per non controllare e non scaricare nulla
	// Ora del giorno (0-23, ora locale) in cui il server, se ha una versione
	// scaricata e nessuno sta giocando, si riavvia per installarla.
	InstallHour int
	// Busy dice se si sta giocando: in quel caso il riavvio si rimanda.
	Busy func() bool
}

// Updater controlla le release su GitHub e scarica le versioni nuove accanto
// all'eseguibile, pronte da installare. Le installa main: allo spegnimento,
// all'avvio o quando l'Updater chiede di riavviare (vedi RestartRequested).
type Updater struct {
	opts   Options
	api    string // URL dell'API della release più recente
	key    string // chiave pubblica che verifica le firme
	client *http.Client

	restart chan struct{} // riceve una richiesta di riavvio per installare

	busy    sync.Mutex // un controllo o un download alla volta
	mu      sync.Mutex // protegge status e pending
	status  Status
	pending *pending // versione nuova trovata e non ancora scaricata
}

// New prepara l'aggiornamento dell'eseguibile. Con Enabled falso (o in una
// build di sviluppo) non controlla e non scarica nulla.
func New(opts Options) *Updater {
	opts.Enabled = opts.Enabled && version.Release()
	if opts.Busy == nil {
		opts.Busy = func() bool { return false }
	}
	u := &Updater{
		opts:    opts,
		api:     "https://api.github.com/repos/" + repo + "/releases/latest",
		key:     publicKey,
		client:  &http.Client{Timeout: 10 * time.Minute},
		restart: make(chan struct{}, 1),
	}
	u.status = Status{
		Enabled: opts.Enabled, Current: version.Number, Staged: Staged(opts.Exe), InstallHour: opts.InstallHour,
	}
	os.Remove(opts.Exe + downloadSuffix) // rimasto da un download interrotto
	return u
}

// Status restituisce lo stato degli aggiornamenti.
func (u *Updater) Status() Status {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.status
}

// RestartRequested riceve un valore quando c'è una versione da installare e
// il momento è adatto: main spegne il server, la installa e lo riavvia.
func (u *Updater) RestartRequested() <-chan struct{} { return u.restart }

// Errori di Check, StartDownload e InstallNow.
var (
	ErrDisabled         = errors.New("aggiornamenti disattivati")
	ErrInProgress       = errors.New("è già in corso un controllo o un download")
	ErrNothingAvailable = errors.New("nessuna versione nuova da scaricare")
	ErrNothingStaged    = errors.New("nessuna versione nuova da installare")
	ErrBusy             = errors.New("si sta giocando: l'aggiornamento si installerà più tardi")
)

// InstallNow chiede di riavviare subito il server per installare la versione
// scaricata, se non si sta giocando.
func (u *Updater) InstallNow() error {
	if u.Status().Staged == "" {
		return ErrNothingStaged
	}
	if u.opts.Busy() {
		return ErrBusy
	}
	u.requestRestart()
	return nil
}

func (u *Updater) requestRestart() {
	select {
	case u.restart <- struct{}{}:
	default: // già chiesto
	}
}

// Run controlla le release poco dopo l'avvio e poi ogni checkEvery; ogni
// installEvery, se c'è una versione scaricata, è l'ora dell'installazione e
// nessuno sta giocando, chiede il riavvio. Termina quando ctx viene annullato.
func (u *Updater) Run(ctx context.Context) {
	if !u.opts.Enabled {
		return
	}
	check := time.NewTimer(firstCheck)
	defer check.Stop()
	install := time.NewTicker(installEvery)
	defer install.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-check.C:
			if err := u.CheckAndDownload(ctx); err != nil && ctx.Err() == nil && err != ErrInProgress {
				log.Printf("aggiornamento: %v", err)
			}
			check.Reset(checkEvery)
		case now := <-install.C:
			if u.Status().Staged != "" && now.Hour() == u.opts.InstallHour && !u.opts.Busy() {
				log.Print("aggiornamento: riavvio notturno per installare la versione nuova")
				u.requestRestart()
			}
		}
	}
}

// release è la parte che ci serve della risposta dell'API di GitHub.
type release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

// Check guarda l'ultima release su GitHub senza scaricarla: se è più recente
// di quella in esecuzione finisce in Status.Available, da scaricare con
// StartDownload. L'esito finisce anche nello stato.
func (u *Updater) Check(ctx context.Context) error {
	if !u.opts.Enabled {
		return ErrDisabled
	}
	if !u.busy.TryLock() {
		return ErrInProgress
	}
	defer u.busy.Unlock()
	return u.done(u.find(ctx))
}

// CheckAndDownload guarda l'ultima release su GitHub e, se è più recente di
// quella in esecuzione, la scarica e la verifica. È il controllo automatico.
func (u *Updater) CheckAndDownload(ctx context.Context) error {
	if !u.opts.Enabled {
		return ErrDisabled
	}
	if !u.busy.TryLock() {
		return ErrInProgress
	}
	defer u.busy.Unlock()
	err := u.find(ctx)
	if err == nil && u.Status().Available != "" {
		err = u.fetch(ctx)
	}
	return u.done(err)
}

// StartDownload comincia a scaricare la versione trovata da Check e torna
// subito: l'avanzamento si segue con Status. Il download si interrompe solo
// se ctx viene annullato.
func (u *Updater) StartDownload(ctx context.Context) error {
	if !u.opts.Enabled {
		return ErrDisabled
	}
	if !u.busy.TryLock() {
		return ErrInProgress
	}
	if u.Status().Available == "" {
		u.busy.Unlock()
		return ErrNothingAvailable
	}
	// La fase si imposta subito, così chi legge lo stato vede il download partito.
	u.setPhase(PhaseDownloading)
	go func() {
		defer u.busy.Unlock()
		if err := u.done(u.fetch(ctx)); err != nil && ctx.Err() == nil {
			log.Printf("aggiornamento: %v", err)
		}
	}()
	return nil
}

func (u *Updater) setPhase(phase string) {
	u.mu.Lock()
	u.status.Phase = phase
	u.mu.Unlock()
}

// done chiude l'operazione in corso registrandone l'esito nello stato.
func (u *Updater) done(err error) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.status.Phase = ""
	u.status.Downloaded = 0
	u.status.Error = ""
	if err != nil {
		u.status.Error = err.Error()
	}
	return err
}

// find legge l'ultima release e, se è da scaricare, la ricorda in u.pending.
func (u *Updater) find(ctx context.Context) error {
	u.setPhase(PhaseChecking)
	var rel release
	body, err := u.get(ctx, u.api, maxSmallSize)
	if err != nil {
		return fmt.Errorf("lettura dell'ultima release: %w", err)
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return fmt.Errorf("risposta di GitHub non valida: %w", err)
	}
	latest := strings.TrimPrefix(rel.TagName, "v")
	u.mu.Lock()
	u.status.Latest = latest
	u.status.Available, u.status.Size, u.pending = "", 0, nil
	staged := u.status.Staged
	u.mu.Unlock()

	var p *pending
	var size int64
	if version.Newer(latest) && staged != latest {
		p = &pending{number: latest, tag: rel.TagName, urls: map[string]string{}}
		name := AssetName(runtime.GOOS, runtime.GOARCH)
		for _, a := range rel.Assets {
			p.urls[a.Name] = a.URL
			if a.Name == name {
				size = a.Size
			}
		}
		for _, n := range []string{name, ChecksumsName, SignatureName} {
			if p.urls[n] == "" {
				return fmt.Errorf("nella release %s manca il file %s", rel.TagName, n)
			}
		}
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.pending = p
	if p != nil {
		u.status.Available, u.status.Size = latest, size
	}
	u.status.CheckedAt = time.Now().UTC().Format(time.DateTime)
	return nil
}

// fetch scarica e verifica la versione in u.pending, e la rende quella da installare.
func (u *Updater) fetch(ctx context.Context) error {
	u.mu.Lock()
	p := u.pending
	u.status.Phase, u.status.Downloaded = PhaseDownloading, 0
	u.mu.Unlock()
	if p == nil {
		return ErrNothingAvailable
	}

	// Prima la firma dei checksum, poi il checksum dell'eseguibile: così un
	// file modificato o corrotto non viene mai installato.
	name := AssetName(runtime.GOOS, runtime.GOARCH)
	sums, err := u.get(ctx, p.urls[ChecksumsName], maxSmallSize)
	if err != nil {
		return fmt.Errorf("download dei checksum: %w", err)
	}
	sig, err := u.get(ctx, p.urls[SignatureName], maxSmallSize)
	if err != nil {
		return fmt.Errorf("download della firma: %w", err)
	}
	if err := verify(u.key, sums, string(sig)); err != nil {
		return err
	}
	want, err := checksumOf(sums, name)
	if err != nil {
		return err
	}
	if err := u.download(ctx, p.urls[name], want); err != nil {
		return fmt.Errorf("download di %s: %w", name, err)
	}
	if err := u.stage(p.number); err != nil {
		return err
	}
	u.mu.Lock()
	u.status.Staged = p.number
	if u.pending == p {
		u.status.Available, u.status.Size, u.pending = "", 0, nil
	}
	u.mu.Unlock()
	log.Printf("aggiornamento: scaricata la versione %s, verrà installata al prossimo riavvio", p.number)
	return nil
}

// get scarica un file piccolo, al massimo limit byte.
func (u *Updater) get(ctx context.Context, url string, limit int64) ([]byte, error) {
	res, err := u.request(ctx, url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("file troppo grande")
	}
	return data, nil
}

func (u *Updater) request(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// GitHub rifiuta le richieste senza User-Agent.
	req.Header.Set("User-Agent", "f1-scoreboard/"+version.Number)
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("risposta %s", res.Status)
	}
	return res, nil
}

// download scarica l'eseguibile in exe.download controllandone lo SHA-256.
func (u *Updater) download(ctx context.Context, url string, want []byte) (err error) {
	path := u.opts.Exe + downloadSuffix
	defer func() {
		if err != nil {
			os.Remove(path)
		}
	}()
	res, err := u.request(ctx, url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.ContentLength > 0 {
		u.mu.Lock()
		u.status.Size = res.ContentLength
		u.mu.Unlock()
	}
	// 0o755: su Linux il file deve essere eseguibile (su Windows non conta).
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, hash, progress{u}), io.LimitReader(res.Body, maxBinarySize+1))
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if n > maxBinarySize {
		return errors.New("file troppo grande")
	}
	if !bytes.Equal(hash.Sum(nil), want) {
		return errors.New("il checksum non corrisponde")
	}
	return nil
}

// progress conta nello stato i byte scaricati, per mostrare l'avanzamento.
type progress struct{ u *Updater }

func (p progress) Write(b []byte) (int, error) {
	p.u.mu.Lock()
	p.u.status.Downloaded += int64(len(b))
	p.u.mu.Unlock()
	return len(b), nil
}

// stage rende il file scaricato la versione da installare. Il numero si scrive
// prima del rename: un .new esiste solo dopo aver verificato tutto.
func (u *Updater) stage(number string) error {
	if err := os.WriteFile(u.opts.Exe+versionSuffix, []byte(number), 0o644); err != nil {
		return err
	}
	return os.Rename(u.opts.Exe+downloadSuffix, u.opts.Exe+newSuffix)
}

// AssetName è il nome dell'eseguibile per un sistema nelle release, uguale a
// quello scritto dal workflow di release.
func AssetName(goos, goarch string) string {
	name := "f1-scoreboard-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// checksumOf cerca in checksums.txt (formato di sha256sum) l'hash di name.
func checksumOf(sums []byte, name string) ([]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(sums))
	for scanner.Scan() {
		// Le righe sono "hash  nome", o "hash *nome" per i file letti in modalità binaria.
		hash, file, ok := strings.Cut(scanner.Text(), " ")
		if ok && len(file) > 0 && file[1:] == name && (file[0] == ' ' || file[0] == '*') {
			return hex.DecodeString(hash)
		}
	}
	return nil, fmt.Errorf("nei checksum manca %s", name)
}

// Staged restituisce la versione scaricata e pronta da installare accanto a
// exe, vuota se non ce n'è. Se il numero non si legge restituisce "?".
func Staged(exe string) string {
	if _, err := os.Stat(exe + newSuffix); err != nil {
		return ""
	}
	number, err := os.ReadFile(exe + versionSuffix)
	if err != nil || len(number) == 0 {
		return "?"
	}
	return strings.TrimSpace(string(number))
}
