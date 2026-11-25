package processfixture

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// StartObservedClock refreshes an independent file observation until joined.
func StartObservedClock(t testing.TB) (string, func()) {
	t.Helper()
	path := filepath.Join(TempDir(t), "clock-observation")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	return path, RefreshObservedClock(t, path)
}

// RefreshObservedClock maintains an existing clock observation until joined.
func RefreshObservedClock(t testing.TB, path string) func() {
	t.Helper()
	now := time.Now()
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatal("refresh clock observation", err)
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
	return func() {
		close(stop)
		if err := <-joined; err != nil {
			t.Error("clock observation updater", err)
		}
	}
}
