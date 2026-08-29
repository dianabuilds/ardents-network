package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestContributorClockObservationOwnerRefreshesUntilStopped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clock.observation")
	if err := os.WriteFile(path, []byte("owned by the Contributor process\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	stop, err := startContributorClockObservation(context.Background(), path, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stop() })
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		info, statErr := os.Stat(path)
		if statErr == nil && info.ModTime().After(old.Add(time.Minute)) {
			if err := stop(); err != nil {
				t.Fatal(err)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("Contributor clock observation was not refreshed")
}

func TestContributorClockObservationOwnerRejectsNonRegularInput(t *testing.T) {
	if _, err := startContributorClockObservation(context.Background(), t.TempDir(), time.Second); err == nil {
		t.Fatal("directory clock observation was accepted")
	}
}
