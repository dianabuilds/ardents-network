package publication

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// These are component phase receipts, not simulated storage hardware or
// evidence of a filesystem's behavior during actual power loss.
func TestPublicationDirectoryCommitOrder(t *testing.T) {
	t.Parallel()
	fixture := newPublicationFixture(t)
	owner, err := Open(fixture.config(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	var phases []string
	owner.root.syncDirectory = func(path string) error {
		switch {
		case filepath.Base(path) == "generations":
			if _, err := os.Stat(generationPath(owner.root.path, publicationGeneration(1))); err != nil {
				t.Fatal(err)
			}
			phases = append(phases, "generation")
		case filepath.Dir(path) == filepath.Join(owner.root.path, "generations"):
			if _, err := os.Stat(filepath.Join(path, "publication.bin")); err != nil {
				t.Fatal(err)
			}
			phases = append(phases, "record-directory")
		case path == owner.root.path:
			if currentExists(path) {
				phases = append(phases, "current")
			} else {
				phases = append(phases, "floor")
			}
		default:
			t.Fatalf("unexpected barrier %s", path)
		}
		return syncPublicationDirectory(path)
	}
	input := fixture.input(t, 1)
	input.Acknowledgement = nil
	_, err = owner.PublishAfterReadiness(t.Context(), input, func(_ context.Context) ([]byte, error) {
		if !reflect.DeepEqual(phases, []string{"floor"}) {
			t.Fatalf("readiness before durable floor: %v", phases)
		}
		return []byte("ready"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"floor", "record-directory", "generation", "current"}; !reflect.DeepEqual(phases, want) {
		t.Fatalf("barriers %v, want %v", phases, want)
	}
	owner.root.syncDirectory = syncPublicationDirectory
}

func TestPublicationRefusesEachAmbiguousCommitPhase(t *testing.T) {
	for phase := 1; phase <= 4; phase++ {
		t.Run(fmt.Sprint(phase), func(t *testing.T) {
			t.Parallel()
			fixture := newPublicationFixture(t)
			root := t.TempDir()
			owner, err := Open(fixture.config(root))
			if err != nil {
				t.Fatal(err)
			}
			failure := errors.New("injected directory barrier failure")
			calls, readinessCalls := 0, 0
			owner.root.syncDirectory = func(path string) error {
				calls++
				if calls == phase {
					return failure
				}
				return syncPublicationDirectory(path)
			}
			input := fixture.input(t, 1)
			input.Acknowledgement = nil
			readiness := func(context.Context) ([]byte, error) { readinessCalls++; return []byte("ready"), nil }
			current, err := owner.PublishAfterReadiness(t.Context(), input, readiness)
			if !errors.Is(err, failure) || len(current.Record) != 0 {
				t.Fatalf("ambiguous phase accepted: %+v %v", current, err)
			}
			if phase == 1 && readinessCalls != 0 {
				t.Fatal("failed floor reached readiness")
			}
			if _, err := owner.Acquire(t.Context()); err == nil {
				t.Fatal("ambiguous generation acquired")
			}
			before := readinessCalls
			if _, err := owner.PublishAfterReadiness(t.Context(), input, readiness); !errors.Is(err, failure) {
				t.Fatalf("retry forgot failed barrier: %v", err)
			}
			if readinessCalls != before {
				t.Fatal("ambiguous retry invoked readiness")
			}
			if _, err := owner.Floor(); !errors.Is(err, failure) {
				t.Fatalf("ambiguous floor advertised: %v", err)
			}
			owner.root.syncDirectory = syncPublicationDirectory
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(fixture.config(root))
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			if floor, err := reopened.Floor(); err != nil || floor != 1 {
				t.Fatalf("reopen lost floor: %d %v", floor, err)
			}
			if _, err := reopened.Publish(t.Context(), fixture.input(t, 1)); err == nil {
				t.Fatal("reopen reused floor")
			}
			if _, err := reopened.Publish(t.Context(), fixture.input(t, 2)); err != nil {
				t.Fatalf("higher generation after reconciliation: %v", err)
			}
		})
	}
}

func TestPublicationWithdrawalRetriesDirectoryBarriers(t *testing.T) {
	for _, phase := range []string{"current", "generation"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			fixture := newPublicationFixture(t)
			owner, err := Open(fixture.config(t.TempDir()))
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			if _, err := owner.Publish(t.Context(), fixture.input(t, 1)); err != nil {
				t.Fatal(err)
			}
			failure := errors.New("withdrawal barrier failed")
			failed := false
			owner.root.syncDirectory = func(path string) error {
				match := path == owner.root.path
				if phase == "generation" {
					match = path == filepath.Join(owner.root.path, "generations")
				}
				if match && !failed {
					failed = true
					return failure
				}
				return syncPublicationDirectory(path)
			}
			if err := owner.Unpublish(t.Context()); !errors.Is(err, failure) {
				t.Fatalf("lost withdrawal error: %v", err)
			}
			if _, err := owner.Acquire(t.Context()); err == nil {
				t.Fatal("failed withdrawal left live publication")
			}
			if owner.root.retiring == nil {
				t.Fatal("failed barrier lost retirement ownership")
			}
			owner.root.syncDirectory = syncPublicationDirectory
			if err := owner.Unpublish(t.Context()); err == nil {
				t.Fatal("repeated withdrawal advertised success")
			}
			if owner.root.retiring != nil {
				t.Fatal("retry failed to settle retirement")
			}
			if floor, err := owner.Floor(); err != nil || floor != 1 {
				t.Fatalf("withdraw lost floor: %d %v", floor, err)
			}
		})
	}
}

func TestPublicationInitializationRetainsBarrierFailures(t *testing.T) {
	for phase := 1; phase <= 5; phase++ {
		t.Run(fmt.Sprint(phase), func(t *testing.T) {
			t.Parallel()
			fixture := newPublicationFixture(t)
			root := t.TempDir()
			failure := errors.New("initialization barrier failed")
			calls := 0
			owner, err := openDurableRootWithSync(fixture.config(root), acquireRootLease, func(path string) error {
				calls++
				if calls == phase {
					return failure
				}
				return syncPublicationDirectory(path)
			})
			if owner != nil || !errors.Is(err, failure) {
				t.Fatalf("initialization accepted failed barrier: %v", err)
			}
			reopened, err := Open(fixture.config(root))
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPublicationRootCreationFlushesAncestorLinks(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	root := filepath.Join(parent, "nested", "publication")
	var paths []string
	if err := createPublicationDirectory(root, func(path string) error {
		paths = append(paths, path)
		return syncPublicationDirectory(path)
	}); err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Dir(parent), parent, filepath.Join(parent, "nested")}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("ancestor barriers %v, want %v", paths, want)
	}
}

func TestPublicationRetainsCommitAndCleanupBarrierErrors(t *testing.T) {
	t.Parallel()
	fixture := newPublicationFixture(t)
	root := t.TempDir()
	owner, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	commitFailure := errors.New("current commit barrier failed")
	cleanupFailure := errors.New("current withdrawal barrier failed")
	calls := 0
	owner.root.syncDirectory = func(path string) error {
		calls++
		if calls == 4 {
			return commitFailure
		}
		if calls == 5 {
			return cleanupFailure
		}
		return syncPublicationDirectory(path)
	}
	_, err = owner.Publish(t.Context(), fixture.input(t, 1))
	if !errors.Is(err, commitFailure) || !errors.Is(err, cleanupFailure) {
		t.Fatalf("lost persistence error: %v", err)
	}
	if _, err := os.Stat(generationPath(root, publicationGeneration(1))); err != nil {
		t.Fatalf("removed generation before durable pointer withdrawal: %v", err)
	}
	if _, err := owner.Acquire(t.Context()); err == nil {
		t.Fatal("ambiguous cleanup exposed signer")
	}
	owner.root.syncDirectory = syncPublicationDirectory
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if floor, err := reopened.Floor(); floor != 1 || err != nil {
		t.Fatalf("reconciliation lost floor: %d %v", floor, err)
	}
}
