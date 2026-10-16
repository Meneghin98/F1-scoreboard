package api

import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

// staticHandler serve i file del frontend con le intestazioni di cache giuste e,
// se il browser accetta gzip, la versione .gz generata durante la build.
func staticHandler(files fs.FS) http.Handler {
	fileServer := http.FileServerFS(files)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}

		// I file in assets/ hanno un hash nel nome che cambia a ogni build: il browser
		// può tenerli per sempre. Gli altri (index.html, favicon) vanno ricontrollati,
		// così dopo un aggiornamento la pagina punta subito ai nuovi assets.
		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}

		if info, err := fs.Stat(files, name+".gz"); err == nil && !info.IsDir() {
			// La risposta cambia in base ad Accept-Encoding: va detto anche alle cache
			// intermedie, sia quando si invia la versione compressa sia quando no.
			w.Header().Set("Vary", "Accept-Encoding")
			if acceptsGzip(r) {
				w.Header().Set("Content-Encoding", "gzip")
				// Il tipo va dedotto dal file originale, non dall'estensione .gz.
				if ctype := mime.TypeByExtension(path.Ext(name)); ctype != "" {
					w.Header().Set("Content-Type", ctype)
				}
				http.ServeFileFS(w, r, files, name+".gz")
				return
			}
		}
		fileServer.ServeHTTP(w, r)
	})
}

// acceptsGzip dice se il client accetta risposte compresse con gzip.
func acceptsGzip(r *http.Request) bool {
	for part := range strings.SplitSeq(r.Header.Get("Accept-Encoding"), ",") {
		encoding, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if strings.EqualFold(strings.TrimSpace(encoding), "gzip") {
			// "gzip;q=0" significa esplicitamente "non accettato".
			return strings.ReplaceAll(params, " ", "") != "q=0"
		}
	}
	return false
}
