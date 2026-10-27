//go:build network_signing_native

package systemd

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"
)

// ManagerIdentity is a native-test projection. Production stopped preparation
// retains its own CheckAbsent/CheckOriginal observation and physical lifetime.
func (s *ForwardingStopped) ManagerIdentity() (string, error) {
	if s == nil || s.original == nil || s.original.reference == nil {
		return "", ErrInput
	}
	return s.original.reference.managerIdentity()
}

func forwardingManagerStand(t *testing.T) {
	t.Helper()
	marker, err := os.ReadFile("/etc/ardents-forwarding-preparation-native-stand")
	if err != nil || string(marker) != "ardents-forwarding-preparation-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("invalid environment: explicit root Forwarding preparation stand required", err)
	}
}

func managerFDCount(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func TestForwardingManagerNativeOriginalConnectionAndDescriptorReturn(t *testing.T) {
	forwardingManagerStand(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	open := func() {
		t.Helper()
		manager, err := OpenForwardingStopped(ctx)
		if err != nil {
			if manager != nil {
				err = errors.Join(err, manager.Close())
			}
			t.Fatal("actual five-slot preflight", err)
		}
		paths, err := manager.CheckAbsent()
		if err != nil || len(paths) == 0 {
			t.Fatal(err)
		}
		paths[0] = "/foreign-detached-path"
		identity, err := manager.ManagerIdentity()
		if err != nil || identity == "" {
			t.Fatal(err)
		}
		if err := manager.CheckOriginal(); err != nil {
			t.Fatal("detached projection modified original manager custody", err)
		}
		copy := *manager
		if err := copy.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := manager.CheckAbsent(); err == nil {
			t.Fatal("copy Close did not deny original observation")
		}
		if err := manager.Close(); err != nil {
			t.Fatal("repeated successful join changed result", err)
		}
	}
	open() // Warm the real runtime/bus before the independent FD inventory.
	before := managerFDCount(t)
	for range 8 {
		open()
		if got := managerFDCount(t); got != before {
			t.Fatal("joined manager retained native descriptors without GC", before, got)
		}
	}
}

func TestForwardingManagerNativeCloseJoinsOriginalObservation(t *testing.T) {
	forwardingManagerStand(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	manager, err := OpenForwardingStopped(ctx)
	if err != nil {
		if manager != nil {
			err = errors.Join(err, manager.Close())
		}
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	manager.original.reference.observe = func() {
		once.Do(func() { close(entered); <-release })
	}
	checked, closed := make(chan error, 1), make(chan error, 1)
	go func() { checked <- manager.CheckOriginal() }()
	<-entered
	cancel()
	copy := *manager
	go func() { closed <- copy.Close() }()
	// The cancellation check precedes physical serialization, even while the
	// original observer is held after its actual manager I/O.
	if err := manager.CheckOriginal(); !errors.Is(err, context.Canceled) {
		t.Fatal("original caller loss waited behind physical I/O", err)
	}
	select {
	case <-closed:
		t.Fatal("Close returned before the original observer joined")
	default:
	}
	close(release)
	if err := <-checked; !errors.Is(err, context.Canceled) {
		t.Fatal("held observation lost original cancellation", err)
	}
	result := <-closed
	if !errors.Is(result, context.Canceled) || manager.Close() != result {
		t.Fatal("copied Close did not retain the joined original result", result)
	}
}
