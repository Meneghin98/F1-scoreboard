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
	u := New(exe, true)
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
	if data, _ := os.ReadFile(u.exe + newSuffix); string(data) != "nuovo" {
		t.Errorf("file scaricato %q", data)
	}
	if Staged(u.exe) != "1.1.0" {
		t.Errorf("Staged = %q", Staged(u.exe))
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
	if _, err := os.Stat(u.exe + newSuffix); err == nil {
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
			if _, err := os.Stat(u.exe + suffix); err == nil {
				t.Errorf("rimasto il file %s", suffix)
			}
		}
	})
}

func TestDevelopmentBuildDoesNotUpdate(t *testing.T) {
	setVersion(t, "")
	u := New(filepath.Join(t.TempDir(), "f1-scoreboard"), true)
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
