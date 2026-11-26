package durable

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestFreshStateRootRefusesEveryPreviousAttempt(t *testing.T) {
	for _, name := range []string{rootLockName, rootMarkerName, ".current-original"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, name)
			raw := []byte("retained original bytes")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if owner, err := OpenFresh(root, Limits{EpochBytes: 1024, RecordBytes: 1024, ClosedProfileBytes: 1024}); err == nil {
				_ = owner.Close()
				t.Fatal("previous State attempt adopted")
			}
			after, err := os.ReadFile(path)
			entries, readErr := os.ReadDir(root)
			if err != nil || readErr != nil || !bytes.Equal(raw, after) || len(entries) != 1 {
				t.Fatal("fresh opening changed original prefix", err, readErr)
			}
		})
	}
	root := t.TempDir()
	limits := Limits{EpochBytes: 1024, RecordBytes: 1024, ClosedProfileBytes: 1024}
	owner, err := OpenFresh(root, limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if second, err := OpenFresh(root, limits); err == nil {
		_ = second.Close()
		t.Fatal("first State claim initialized again")
	}
}
