//go:build windows

package disk

import "golang.org/x/sys/windows"

// UsageOf restituisce lo spazio del disco che contiene la cartella path.
func UsageOf(path string) (Usage, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return Usage{}, err
	}
	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &free, &total, &totalFree); err != nil {
		return Usage{}, err
	}
	return Usage{FreeBytes: free, TotalBytes: total}, nil
}
