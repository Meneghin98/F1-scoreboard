// Package disk dice quanto spazio c'è sul disco che contiene una cartella.
// Il modo di chiederlo dipende dal sistema operativo: c'è un file per ognuno
// (disk_windows.go, disk_linux.go), scelto in compilazione dai build tag.
package disk

// Usage è lo spazio del disco (o della partizione) che contiene una cartella.
type Usage struct {
	FreeBytes  uint64 `json:"freeBytes"` // spazio disponibile per il server
	TotalBytes uint64 `json:"totalBytes"`
}
