package disk

import (
	"errors"
	"testing"
)

func TestUsageOf(t *testing.T) {
	u, err := UsageOf(t.TempDir())
	if errors.Is(err, errors.ErrUnsupported) {
		t.Skip("non disponibile su questo sistema operativo")
	}
	if err != nil {
		t.Fatal(err)
	}
	if u.FreeBytes == 0 || u.TotalBytes < u.FreeBytes {
		t.Fatalf("spazio %+v", u)
	}
}
