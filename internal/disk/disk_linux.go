//go:build linux

package disk

import "golang.org/x/sys/unix"

// UsageOf restituisce lo spazio del disco che contiene la cartella path.
func UsageOf(path string) (Usage, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return Usage{}, err
	}
	// I blocchi si contano in unità di Frsize (come fa df). I tipi dei campi
	// cambiano con l'architettura (int32 sul Raspberry a 32 bit): da qui le conversioni.
	block := uint64(st.Frsize)
	if block == 0 {
		block = uint64(st.Bsize)
	}
	return Usage{FreeBytes: uint64(st.Bavail) * block, TotalBytes: uint64(st.Blocks) * block}, nil
}
