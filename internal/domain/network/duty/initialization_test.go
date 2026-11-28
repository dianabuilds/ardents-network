package duty

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestInitializeCreatesOnlyFreshParticipationHistory(t *testing.T) {
	root := ownedRootFixture(t)
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	config := Config{Root: root, Clock: time.Now}
	if err := Initialize(t.Context(), config); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Initialize(t.Context(), config); err == nil {
		t.Fatal("existing history initialized twice")
	}
	after, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("repeated initialization changed history", err)
	}
	owner, err := Open(Config{Root: root, Clock: time.Now})
	if err != nil {
		t.Fatal("strict retained opening failed", err)
	}
	conflict, err := owner.Conflict([32]byte{1}, [32]byte{2})
	if err != nil || conflict {
		t.Fatal("empty initialization granted a duty", conflict, err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestInitializeRefusesPartialParticipationWithoutRepair(t *testing.T) {
	for _, name := range []string{rootLockName, rootMarkerName, ".stage-original", "watermark"} {
		t.Run(name, func(t *testing.T) {
			root := ownedRootFixture(t)
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, name)
			original := []byte("original partial bytes")
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			if err := Initialize(t.Context(), Config{Root: root, Clock: time.Now}); err == nil {
				t.Fatal("partial history adopted")
			}
			raw, err := os.ReadFile(path)
			entries, readErr := os.ReadDir(root)
			if err != nil || readErr != nil || !bytes.Equal(raw, original) || len(entries) != 1 {
				t.Fatal("partial history changed", err, readErr)
			}
		})
	}
	root := ownedRootFixture(t)
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := Initialize(ctx, Config{Root: root, Clock: time.Now}); err != context.Canceled {
		t.Fatal("canceled initialization", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("canceled caller touched history", err)
	}
}

func TestConcurrentInitializationHasOnlyOneParticipationCreator(t *testing.T) {
	root := ownedRootFixture(t)
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var done sync.WaitGroup
	for range 2 {
		done.Go(func() {
			<-start
			results <- Initialize(t.Context(), Config{Root: root, Clock: time.Now})
		})
	}
	close(start)
	done.Wait()
	close(results)
	accepted := 0
	for err := range results {
		if err == nil {
			accepted++
		}
	}
	if accepted != 1 {
		t.Fatal("number of original creators", accepted)
	}
	owner, err := Open(Config{Root: root, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	if owner.state.Generation != 1 {
		t.Fatal("competing creator advanced history", owner.state.Generation)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
}
