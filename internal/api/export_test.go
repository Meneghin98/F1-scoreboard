package api

import "testing"

// SetInstallDir cambia la cartella di installazione fino alla fine del test.
func SetInstallDir(t *testing.T, dir string) {
	old := installDir
	installDir = func() string { return dir }
	t.Cleanup(func() { installDir = old })
}
