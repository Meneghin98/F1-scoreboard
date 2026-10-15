// Package web incorpora i file del frontend nel binario.
package web

import (
	"embed"
	"io/fs"
)

//go:embed public
var files embed.FS

// Files restituisce i file statici del frontend, con public/ come radice.
func Files() fs.FS {
	sub, err := fs.Sub(files, "public")
	if err != nil {
		panic(err) // impossibile: la cartella è incorporata a compile time
	}
	return sub
}
