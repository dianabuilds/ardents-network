package artifacts

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestObservedInputCloseDeniesBeforeJoiningOriginalUses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "original")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	inputs := NewObservedInputs(ctx, nil)
	if _, err := inputs.Acquire(path, 1024, true); err != nil {
		t.Fatal(err)
	}
	inputs.Watch()
	copied := inputs
	// Hold the native-observation serializer after a genuine original read.
	// Cancellation must not acquire it; Close must retain descriptors until the
	// already admitted use finishes. This is a scheduling control, not State.
	inputs.ioMu.Lock()
	checked := make(chan error, 1)
	go func() { checked <- copied.Check() }()
	cancel()
	closed := make(chan error, 1)
	go func() { closed <- copied.Close() }()
	<-inputs.Context().Done()
	if err := inputs.Check(); err == nil {
		t.Fatal("canceled original accepted work")
	}
	inputs.ioMu.Unlock()
	if err := <-checked; err == nil {
		t.Fatal("late observation accepted canceled generation")
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	if err := inputs.Close(); err != nil {
		t.Fatal("copied Close disagrees", err)
	}
	for _, f := range inputs.Files() {
		if _, err := f.Descriptor().Stat(); !errors.Is(err, os.ErrClosed) {
			t.Fatal("original descriptor survived completed Close", err)
		}
		for _, b := range f.Bytes() {
			if b != 0 {
				t.Fatal("original bytes survived joined Close")
			}
		}
	}
}
