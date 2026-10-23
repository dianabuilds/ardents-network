//go:build linux && network_signing_native

package infrastructure

import (
	"context"
	"errors"
	"os"
	"reflect"
	"runtime/debug"
	"syscall"
	"testing"
)

// The canonical kernel control establishes cancellation at actual Reload. This
// separate coordinator terminal-owner control checks the in-process cause chain
// using real original writers, native failures and a foreign flock, without
// supplying any accepting manager, Release or semantic-history observation.
func TestSourceReplacementNativeClosePreservesFailureAndJoins(t *testing.T) {
	previousGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previousGC)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	births, path := nativeSourceBirths(t, ctx)
	if err := births.write(path, []byte("original finite coordinator write\n"), 0600, 0, 0, ctx.Err); err != nil {
		_ = births.close()
		t.Fatal(err)
	}
	probe, err := os.Open(births.journal.path)
	if err != nil {
		_ = births.close()
		t.Fatal(err)
	}
	defer probe.Close()
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		_ = births.close()
		t.Fatal("original coordinator lease not retained", err)
	}
	watched := map[string]bool{path: true, births.journal.path: true}
	original := errors.New("original coordinator failure")
	operation := &sourceReplacement{ctx: ctx, births: births, terminal: original}
	// These are actual descriptor closes, not substituted successful neighbors
	// or manufactured Close returns. The coordinator must retain their later
	// independent native failures together with its already definite failure.
	if err := births.files[0].file.Close(); err != nil {
		_ = births.close()
		t.Fatal(err)
	}
	cancel()
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		_ = births.close()
		t.Fatal("physical leaf failure released original journal lease early", err)
	}
	result := operation.close()
	if !errors.Is(result, original) || !errors.Is(result, context.Canceled) || !errors.Is(result, os.ErrClosed) {
		t.Fatal("coordinator lost original/cancellation/physical cleanup causes", result)
	}
	if !births.closed || births.terminal == nil || !errors.Is(operation.terminal, original) {
		t.Fatal("coordinator failed to retain its terminal writer state", result)
	}
	expected := map[string]int{births.journal.path: 1} // Only the independent probe remains.
	if got := sourceHistoryDescriptors(t, watched); !reflect.DeepEqual(got, expected) {
		t.Fatal("coordinator returned with original native descriptors before GC", expected, got)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal("coordinator returned before original writer lease release", err)
	}
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
}
