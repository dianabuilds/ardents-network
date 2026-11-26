//go:build !linux && !darwin

package instance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The Unix-family custody port (#863, #691) keeps the remaining target
// platforms on the typed refusal: private host-root custody is unavailable
// before any key or filesystem state is created.
func TestUnsupportedPlatformCustodyRefusesBeforeRootCreation(t *testing.T) {
	before := time.Now().UTC().Truncate(time.Second)
	config := Config{Root: filepath.Join(t.TempDir(), "instance"), NetworkID: [32]byte{3}, NotBefore: before, NotAfter: before.Add(time.Hour)}
	root, err := Initialize(context.Background(), config)
	if root != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatal("unsupported platform accepted Instance custody", root, err)
	}
	opened, err := Open(context.Background(), config.Root)
	if opened != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatal("unsupported platform opened Instance custody", opened, err)
	}
	entries, err := os.ReadDir(filepath.Dir(config.Root))
	if err != nil || len(entries) != 0 {
		t.Fatal("refusal created filesystem state", entries, err)
	}
}
