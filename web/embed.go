// Package web incorpora nel binario il frontend compilato da Vite (npm run build).
package web

import (
	"embed"
	"io/fs"
)

// dist/ contiene sempre almeno .gitkeep (versionato, e ricopiato da public/
// a ogni build), così il backend compila anche senza aver buildato il frontend.
//
//go:embed all:dist
var files embed.FS

// Files restituisce i file statici del frontend, con dist/ come radice.
func Files() fs.FS {
	sub, err := fs.Sub(files, "dist")
	if err != nil {
		panic(err) // impossibile: la cartella è incorporata a compile time
	}
	return sub
}
