package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"f1-scoreboard/internal/version"
)

// fakeGitHub simula l'API delle release e i file da scaricare.
type fakeGitHub struct {
	tag   string
	files map[string][]byte
	srv   *httptest.Server
}

func newFakeGitHub(t *testing.T, tag string, binary []byte, privateKey string) *fakeGitHub {
	t.Helper()
	name := AssetName(runtime.GOOS, runtime.GOARCH)
	sum := sha256.Sum256(binary)
	sums := []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n" + "00  altro-file\n")
	sig, err := Sign(privateKey, sums)
	if err != nil {
		t.Fatal(err)
	}
	g := &fakeGitHub{tag: tag, files: map[string][]byte{
		name: binary, ChecksumsName: sums, SignatureName: []byte(sig + "\n"),
	}}
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/latest" {
			rel := map[string]any{"tag_name": g.tag}
			var assets []map[string]string
			for n := range g.files {
				assets = append(assets, map[string]string{"name": n, "browser_download_url": g.srv.URL + "/files/" + n})
			}
			rel["assets"] = assets
			json.NewEncoder(w).Encode(rel)
			return
		}
		data, ok := g.files[filepath.Base(r.URL.Path)]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	}))
	t.Cleanup(g.srv.Close)
	return g
}

func newTestUpdater(t *testing.T, g *fakeGitHub, publicKey string) *Updater {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "f1-scoreboard.exe")
	if err := os.WriteFile(exe, []byte("vecchio"), 0o755); err != nil {
		t.Fatal(err)
	}
	u := New(Options{Exe: exe, Enabled: true})
	u.api, u.key = g.srv.URL+"/latest", publicKey
	return u
}

func setVersion(t *testing.T, n string) {
	old := version.Number
	version.Number = n
	t.Cleanup(func() { version.Number = old })
}

func TestCheckDownloadsNewVersion(t *testing.T) {
	setVersion(t, "1.0.0")
	private, public, _ := GenerateKey()
	g := newFakeGitHub(t, "v1.1.0", []byte("nuovo"), private)
	u := newTestUpdater(t, g, public)

	if err := u.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	st := u.Status()
	if st.Latest != "1.1.0" || st.Staged != "1.1.0" || st.Error != "" || st.CheckedAt == "" {
		t.Errorf("stato %+v", st)
	}
	if data, _ := os.ReadFile(u.opts.Exe + newSuffix); string(data) != "nuovo" {
		t.Errorf("file scaricato %q", data)
	}
	if Staged(u.opts.Exe) != "1.1.0" {
		t.Errorf("Staged = %q", Staged(u.opts.Exe))
	}

	// Un secondo controllo con la stessa versione non riscarica nulla.
	delete(g.files, AssetName(runtime.GOOS, runtime.GOARCH))
	if err := u.Check(context.Background()); err != nil {
		t.Errorf("secondo controllo: %v", err)
	}
}

func TestCheckSameVersion(t *testing.T) {
	setVersion(t, "1.1.0")
	private, public, _ := GenerateKey()
	g := newFakeGitHub(t, "v1.1.0", []byte("nuovo"), private)
	u := newTestUpdater(t, g, public)
	if err := u.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st := u.Status(); st.Latest != "1.1.0" || st.Staged != "" {
		t.Errorf("stato %+v", st)
	}
	if _, err := os.Stat(u.opts.Exe + newSuffix); err == nil {
		t.Error("scaricata una versione uguale a quella in esecuzione")
	}
}

func TestCheckRejectsBadFiles(t *testing.T) {
	setVersion(t, "1.0.0")
	private, public, _ := GenerateKey()

	t.Run("firma di un'altra chiave", func(t *testing.T) {
		other, _, _ := GenerateKey()
		g := newFakeGitHub(t, "v1.1.0", []byte("nuovo"), other)
		u := newTestUpdater(t, g, public)
		if err := u.Check(context.Background()); err == nil {
			t.Error("accettata una firma sbagliata")
		}
		if st := u.Status(); st.Staged != "" || st.Error == "" {
			t.Errorf("stato %+v", st)
		}
	})

	t.Run("eseguibile modificato", func(t *testing.T) {
		g := newFakeGitHub(t, "v1.1.0", []byte("nuovo"), private)
		g.files[AssetName(runtime.GOOS, runtime.GOARCH)] = []byte("modificato")
		u := newTestUpdater(t, g, public)
		if err := u.Check(context.Background()); err == nil {
			t.Error("accettato un eseguibile con checksum sbagliato")
		}
		for _, suffix := range []string{newSuffix, downloadSuffix} {
			if _, err := os.Stat(u.opts.Exe + suffix); err == nil {
				t.Errorf("rimasto il file %s", suffix)
			}
		}
	})
}

func TestDevelopmentBuildDoesNotUpdate(t *testing.T) {
	setVersion(t, "")
	u := New(Options{Exe: filepath.Join(t.TempDir(), "f1-scoreboard"), Enabled: true})
	if u.Status().Enabled || u.Check(context.Background()) == nil {
		t.Error("una build di sviluppo non deve aggiornarsi")
	}
}

func TestChecksumOf(t *testing.T) {
	sums := []byte("aabb  uno\nccdd *due\n")
	if h, err := checksumOf(sums, "due"); err != nil || hex.EncodeToString(h) != "ccdd" {
		t.Errorf("due: %x %v", h, err)
	}
	if _, err := checksumOf(sums, "tre"); err == nil {
		t.Error("trovato un file che non c'è")
	}
}

func TestInstall(t *testing.T) {
	setVersion(t, "1.0.0")
	private, public, _ := GenerateKey()
	g := newFakeGitHub(t, "v1.1.0", []byte("nuovo"), private)
	u := newTestUpdater(t, g, public)
	exe := u.opts.Exe

	if n, err := Install(exe); n != "" || err != nil {
		t.Fatalf("senza versione scaricata: %q %v", n, err)
	}
	if err := u.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Un .old di un aggiornamento precedente viene sostituito.
	os.WriteFile(exe+oldSuffix, []byte("vecchissimo"), 0o755)
	n, err := Install(exe)
	if n != "1.1.0" || err != nil {
		t.Fatalf("Install = %q %v", n, err)
	}
	for path, want := range map[string]string{exe: "nuovo", exe + oldSuffix: "vecchio"} {
		if data, _ := os.ReadFile(path); string(data) != want {
			t.Errorf("%s contiene %q, voglio %q", filepath.Base(path), data, want)
		}
	}
	if Staged(exe) != "" {
		t.Error("dopo l'installazione resta una versione da installare")
	}
}

func TestInstallNow(t *testing.T) {
	setVersion(t, "1.0.0")
	private, public, _ := GenerateKey()
	g := newFakeGitHub(t, "v1.1.0", []byte("nuovo"), private)
	u := newTestUpdater(t, g, public)
	busy := false
	u.opts.Busy = func() bool { return busy }

	if err := u.InstallNow(); err != ErrNothingStaged {
		t.Errorf("senza versione scaricata: %v", err)
	}
	if err := u.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	busy = true
	if err := u.InstallNow(); err != ErrBusy {
		t.Errorf("mentre si gioca: %v", err)
	}
	busy = false
	if err := u.InstallNow(); err != nil {
		t.Fatal(err)
	}
	u.InstallNow() // una seconda richiesta non blocca
	select {
	case <-u.RestartRequested():
	default:
		t.Error("riavvio non richiesto")
	}
}

func TestInstallDiscardsOlderVersion(t *testing.T) {
	setVersion(t, "1.2.0")
	exe := filepath.Join(t.TempDir(), "f1-scoreboard")
	os.WriteFile(exe, []byte("in uso"), 0o755)
	os.WriteFile(exe+newSuffix, []byte("più vecchio"), 0o755)
	os.WriteFile(exe+versionSuffix, []byte("1.1.0"), 0o644)
	if n, err := Install(exe); n != "" || err != nil {
		t.Errorf("Install = %q %v", n, err)
	}
	if data, _ := os.ReadFile(exe); string(data) != "in uso" || Staged(exe) != "" {
		t.Errorf("eseguibile %q, da installare %q", data, Staged(exe))
	}
}
