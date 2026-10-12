//go:build linux && node_process_native

package processresource

import (
	"context"
	"errors"
	"os"
	"runtime"
	"testing"
)

// This selected native test requires the actual sole-process leaf profile.
// Missing placement is an invalid environment, never a successful skip. The
// real kernel, proc, Go runtime and original descriptors supply every success.
func TestOriginalNativeProcessProfile(t *testing.T) {
	guard, err := Open(context.Background(), Profile)
	if err != nil {
		t.Fatal("selected native process environment unavailable:", err)
	}
	observation, err := guard.Check()
	if err != nil || observation.RSSBytes == 0 || observation.MemoryBytes == 0 || observation.Threads == 0 || observation.FDs == 0 || observation.CPUWindowStarted.IsZero() {
		t.Fatal("genuine native evidence unavailable", err)
	}
	copy := *guard
	prior := runtime.GOMAXPROCS(2)
	_, first := guard.Check()
	runtime.GOMAXPROCS(prior)
	if first == nil {
		t.Fatal("weakened original Go placement accepted")
	}
	_, later := copy.Check()
	if later != first {
		t.Fatal("restored placement erased original failure")
	}
	if err := copy.Close(); err != nil {
		t.Fatal(err)
	}
	if guard.Close() != copy.Close() {
		t.Fatal("copied native owner changed close result")
	}
	if _, err := guard.Check(); err == nil {
		t.Fatal("closed original pins admitted work")
	}
}

func TestOriginalNativeEvidenceLossRetainsActualCloseFailure(t *testing.T) {
	guard, err := Open(context.Background(), Profile)
	if err != nil {
		t.Fatal("selected native process environment unavailable:", err)
	}
	if err := guard.original.native.group.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := guard.Check(); err == nil {
		t.Fatal("lost actual cgroup pin supplied admission")
	}
	first := guard.Close()
	if !errors.Is(first, os.ErrClosed) {
		t.Fatal("native double-close failure was suppressed")
	}
	if guard.Close() != first {
		t.Fatal("late Close erased actual native failure")
	}
}

func TestOriginalNativeManagedMemoryPressureRetires(t *testing.T) {
	guard, err := Open(context.Background(), Profile)
	if err != nil {
		t.Fatal("selected native process environment unavailable:", err)
	}
	t.Cleanup(func() { _ = guard.Close(); runtime.GC() })
	// Retain real resident Go memory above the selected soft limit, while below
	// the kernel emergency limit. This is actual process pressure, not Measure.
	payload := make([]byte, 513<<20)
	for index := 0; index < len(payload); index += os.Getpagesize() {
		payload[index] = 1
	}
	observation, first := guard.Check()
	runtime.KeepAlive(payload)
	if observation.GoMemoryBytes < 512<<20 || first == nil {
		t.Fatal("actual managed-memory pressure admitted work", observation.GoMemoryBytes, first)
	}
	payload = nil
	runtime.GC()
	if _, later := guard.Check(); later != first {
		t.Fatal("released pressure revived the original owner")
	}
}
