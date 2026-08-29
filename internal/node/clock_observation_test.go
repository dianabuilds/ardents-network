package node

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestContributorClockObservationRefreshesUntilStopped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clock.observation")
	if err := os.WriteFile(path, []byte("owned by the Contributor process\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	ticks := make(chan time.Time, 1)
	stop, err := startClockObservation(context.Background(), path, time.Second, clockObservationConfig{
		now:       func() time.Time { return time.Now() },
		newTicker: func(time.Duration) (<-chan time.Time, func()) { return ticks, func() {} },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stop(); err != nil {
			t.Errorf("stop Contributor clock observation: %v", err)
		}
	})
	ticks <- time.Now()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		info, statErr := os.Stat(path)
		if statErr == nil && info.ModTime().After(old.Add(time.Minute)) {
			if err := stop(); err != nil {
				t.Fatal(err)
			}
			if err := stop(); err != nil {
				t.Fatal("idempotent stop:", err)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("Contributor clock observation was not refreshed")
}

func TestContributorClockObservationRejectsNonRegularInput(t *testing.T) {
	if _, err := StartContributorClockObservation(context.Background(), t.TempDir(), time.Second); err == nil {
		t.Fatal("directory clock observation was accepted")
	}
}
