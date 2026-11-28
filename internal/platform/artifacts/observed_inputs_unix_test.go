//go:build linux || darwin

package artifacts

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
)

func TestObservedInputCloseDeniesBeforeJoiningOriginalUses(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "original")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		readReached := make(chan *os.File)
		releaseRead := make(chan struct{})
		observeChecks := false
		inputs := NewObservedInputs(ctx, func(action string, file *os.File) {
			if action == "check" && observeChecks {
				readReached <- file
				<-releaseRead
			}
		})
		original, err := inputs.Acquire(path, 1024, true)
		if err != nil {
			_ = inputs.Close()
			t.Fatal(err)
		}

		// An explicit Check holds the admitted use after its real descriptor
		// read. This isolates its join from the periodic watcher's own join.
		observeChecks = true
		copied := inputs
		checked := make(chan error, 1)
		go func() { checked <- copied.Check() }()
		if observed := <-readReached; observed != original.Descriptor() {
			t.Error("check did not read the original descriptor")
		}

		cancel()
		closed := make(chan struct{})
		var closeErr error
		go func() {
			closeErr = copied.Close()
			close(closed)
		}()
		synctest.Wait()

		select {
		case <-closed:
			t.Error("Close returned before the original read finished")
		default:
		}
		if info, err := original.Descriptor().Stat(); err != nil || !os.SameFile(original.Info(), info) {
			t.Errorf("original descriptor unavailable during an admitted read: %v", err)
		}
		if got := string(original.Bytes()); got != "original" {
			t.Errorf("retained bytes changed while the original read was active: %q", got)
		}
		if err := inputs.Check(); err == nil {
			t.Error("canceled original accepted new work")
		}

		close(releaseRead)
		if err := <-checked; err == nil {
			t.Error("late observation accepted the canceled original")
		}
		<-closed
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		if err := inputs.Close(); err != nil {
			t.Fatal("copied Close disagrees", err)
		}
		if _, err := original.Descriptor().Stat(); !errors.Is(err, os.ErrClosed) {
			t.Fatal("original descriptor survived completed Close", err)
		}
		for _, b := range original.Bytes() {
			if b != 0 {
				t.Fatal("original bytes survived joined Close")
			}
		}
	})
}
