package api

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func gzipBytes(t *testing.T, s string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write([]byte(s))
	zw.Close()
	return buf.Bytes()
}

func TestStaticHandler(t *testing.T) {
	const js = "console.log('ciao')"
	files := fstest.MapFS{
		"index.html":            {Data: []byte("<html></html>")},
		"favicon.svg":           {Data: []byte("<svg/>")},
		"assets/app-1234.js":    {Data: []byte(js)},
		"assets/app-1234.js.gz": {Data: gzipBytes(t, js)},
	}
	handler := staticHandler(files)

	get := func(path, acceptEncoding string) *http.Response {
		req := httptest.NewRequest("GET", path, nil)
		if acceptEncoding != "" {
			req.Header.Set("Accept-Encoding", acceptEncoding)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Result()
	}

	t.Run("asset compresso", func(t *testing.T) {
		res := get("/assets/app-1234.js", "gzip, deflate, br")
		if res.StatusCode != 200 || res.Header.Get("Content-Encoding") != "gzip" {
			t.Fatalf("status %d, encoding %q", res.StatusCode, res.Header.Get("Content-Encoding"))
		}
		if !strings.Contains(res.Header.Get("Content-Type"), "javascript") {
			t.Errorf("Content-Type %q", res.Header.Get("Content-Type"))
		}
		if res.Header.Get("Cache-Control") != "public, max-age=31536000, immutable" {
			t.Errorf("Cache-Control %q", res.Header.Get("Cache-Control"))
		}
		if res.Header.Get("Vary") != "Accept-Encoding" {
			t.Errorf("Vary %q", res.Header.Get("Vary"))
		}
		zr, err := gzip.NewReader(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if body, _ := io.ReadAll(zr); string(body) != js {
			t.Errorf("contenuto decompresso %q", body)
		}
	})

	t.Run("asset senza gzip", func(t *testing.T) {
		for _, enc := range []string{"", "br", "gzip;q=0", "identity"} {
			res := get("/assets/app-1234.js", enc)
			body, _ := io.ReadAll(res.Body)
			if res.Header.Get("Content-Encoding") != "" || string(body) != js {
				t.Errorf("Accept-Encoding %q: encoding %q, corpo %q", enc, res.Header.Get("Content-Encoding"), body)
			}
			if res.Header.Get("Vary") != "Accept-Encoding" {
				t.Errorf("Accept-Encoding %q: manca Vary", enc)
			}
		}
	})

	t.Run("index.html da ricontrollare", func(t *testing.T) {
		for _, path := range []string{"/", "/favicon.svg"} {
			res := get(path, "gzip")
			if res.StatusCode != 200 || res.Header.Get("Cache-Control") != "no-cache" {
				t.Errorf("%s: status %d, Cache-Control %q", path, res.StatusCode, res.Header.Get("Cache-Control"))
			}
			if res.Header.Get("Content-Encoding") != "" {
				t.Errorf("%s: compresso senza una versione .gz", path)
			}
		}
	})

	t.Run("file inesistente", func(t *testing.T) {
		if res := get("/assets/nope.js", "gzip"); res.StatusCode != 404 {
			t.Errorf("status %d", res.StatusCode)
		}
	})

	t.Run("percorso con ..", func(t *testing.T) {
		if res := get("/assets/../index.html", "gzip"); res.StatusCode == 500 {
			t.Errorf("status %d", res.StatusCode)
		}
	})
}

func TestAcceptsGzip(t *testing.T) {
	for header, want := range map[string]bool{
		"":                  false,
		"gzip":              true,
		"GZIP":              true,
		"deflate, gzip, br": true,
		"gzip;q=0.5":        true,
		"gzip;q=0":          false,
		"gzip; q=0":         false,
		"br":                false,
		"x-gzip-custom":     false,
	} {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", header)
		if got := acceptsGzip(req); got != want {
			t.Errorf("Accept-Encoding %q: %v, atteso %v", header, got, want)
		}
	}
}
