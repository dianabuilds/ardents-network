package recoverysmoke

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestReplacementGateWaitCoversFrozenPacing(t *testing.T) {
	isolated, err := pacedGateWait(0, 17*16_381, "20ms")
	if err != nil || isolated <= 15*time.Second || isolated > time.Minute {
		t.Fatalf("isolated gate wait = %s, %v", isolated, err)
	}
	sequential, err := pacedGateWait(0, 64*16_381, "2350ms")
	if err != nil || sequential <= 150*time.Second || sequential > 4*time.Minute {
		t.Fatalf("sequential gate wait = %s, %v", sequential, err)
	}
	if _, err := pacedGateWait(16_381, 16_381, "20ms"); err == nil {
		t.Fatal("non-increasing gate schedule was accepted")
	}
}

func TestWaitGateFileUsesProvidedBound(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "client.ready")
	if err := os.WriteFile(path, []byte("16381\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	observer := dockerObserver{}
	got, err := observer.waitGateFile(context.Background(), path, 16_381, 100*time.Millisecond)
	if err != nil || got != 16_381 {
		t.Fatalf("gate = %d, %v", got, err)
	}
	missing := path + ".missing"
	if _, err := observer.waitGateFile(context.Background(), missing, 16_381, time.Millisecond); err == nil ||
		!strings.Contains(err.Error(), filepath.Base(missing)) || !strings.Contains(err.Error(), "16381") {
		t.Fatalf("missing gate failure lacks actionable context: %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := observer.waitGateFile(canceled, missing, 16_381, time.Second); err == nil ||
		!strings.Contains(err.Error(), filepath.Base(missing)) || !strings.Contains(err.Error(), "16381") {
		t.Fatalf("gate cancellation lacks actionable context: %v", err)
	}
	if _, err := observer.waitGateFile(context.Background(), root, 16_381, time.Second); err == nil {
		t.Fatal("gate read error was hidden by polling")
	}
}

func TestResetSequentialGatesRemovesPriorCellState(t *testing.T) {
	root := t.TempDir()
	offsets := []uint32{17 * 16_381, 64 * 16_381}
	for _, role := range []string{"client", "publisher"} {
		for _, offset := range offsets {
			prefix := filepath.Join(root, role+".ready."+strconv.FormatUint(uint64(offset), 10))
			for _, path := range []string{prefix, prefix + ".pending",
				filepath.Join(root, role+".release."+strconv.FormatUint(uint64(offset), 10))} {
				if err := os.WriteFile(path, []byte("stale\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	unrelated := filepath.Join(root, "unrelated")
	if err := os.WriteFile(unrelated, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := resetSequentialGates(root, offsets); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "unrelated" {
		t.Fatalf("gate reset retained stale state: %v, %v", entries, err)
	}
}

func TestResetSequentialGatesPreservesRemovalFailure(t *testing.T) {
	root := t.TempDir()
	offset := uint32(17 * 16_381)
	ready := filepath.Join(root, "client.ready."+strconv.FormatUint(uint64(offset), 10))
	if err := os.Mkdir(ready, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ready, "owned"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := resetSequentialGates(root, []uint32{offset}); err == nil ||
		!strings.Contains(err.Error(), ready) {
		t.Fatalf("gate reset hid removal failure: %v", err)
	}
}

func TestResetRecoveryGatesRemovesEveryOwnedShapeAndPreservesErrors(t *testing.T) {
	root := t.TempDir()
	names := []string{"client.ready", "client.ready.pending", "client.release",
		"publisher.ready", "publisher.ready.pending", "publisher.release"}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(root, name), []byte("stale"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := resetRecoveryGates(root); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("owned gate state %s remains: %v", name, err)
		}
	}
	blocked := filepath.Join(root, "client.ready")
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blocked, "owned"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := resetRecoveryGates(root); err == nil || !strings.Contains(err.Error(), blocked) {
		t.Fatalf("recovery gate reset hid removal failure: %v", err)
	}
}
