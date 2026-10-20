//go:build !windows && !linux

package disk

import "errors"

// UsageOf su questo sistema operativo non è disponibile.
func UsageOf(path string) (Usage, error) {
	return Usage{}, errors.ErrUnsupported
}
