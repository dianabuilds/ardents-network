//go:build linux && node_process_native

package node

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// StartObservedClock refreshes an independent file observation until joined.
func StartObservedClock(t testing.TB) (string, func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "clock-observation")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	stop, joined := make(chan struct{}), make(chan error, 1)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				joined <- nil
				return
			case now := <-ticker.C:
				if err := os.Chtimes(path, now, now); err != nil {
					joined <- err
					return
				}
			}
		}
	}()
	return path, func() {
		close(stop)
		if err := <-joined; err != nil {
			t.Error("clock observation updater", err)
		}
	}
}
