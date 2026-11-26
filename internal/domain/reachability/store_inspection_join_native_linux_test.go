//go:build linux && network_signing_native

package reachability

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func descriptorInspectionNativeEmpty(t *testing.T) StoreConfig {
	t.Helper()
	if os.Geteuid() != 0 || os.Getenv("ARDENTS_DESCRIPTOR_STORE_INSPECTION_NATIVE") != "1" {
		t.Fatal("invalid environment: explicit root Store inspection stand required")
	}
	parent, err := os.MkdirTemp("/var/lib", "d548-store-join-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(parent); err != nil {
			t.Error(err)
		}
	})
	config := StoreConfig{Root: filepath.Join(parent, "store"), Network: [32]byte{1}}
	store, err := OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := filepath.WalkDir(config.Root, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, 988, 988)
	}); err != nil {
		t.Fatal(err)
	}
	return config
}

func TestDescriptorStoreInspectionNativeCloseJoinsHeldReaderAndRetainsLease(t *testing.T) {
	config := descriptorInspectionNativeEmpty(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second*10)
	defer cancel()
	h, err := OpenStoreInspection(ctx, config, 988, 988)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	s := h.original
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	s.observe = func(stage string, _ *os.File) {
		if stage == "read" {
			select {
			case <-entered:
			default:
				close(entered)
			}
			<-release
		}
	}
	checked, closed := make(chan error, 1), make(chan error, 1)
	go func() { checked <- h.Check() }()
	<-entered
	go func() { closed <- h.Close() }()
	until := time.Now().Add(time.Second)
	for {
		s.mu.Lock()
		denied := s.closed
		s.mu.Unlock()
		if denied {
			break
		}
		if time.Now().After(until) {
			t.Fatal("Close failed to seal original readers")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := s.lease.file.Stat(); err != nil {
		t.Fatal("original lease closed before held reader joined", err)
	}
	select {
	case err := <-closed:
		t.Fatal("Close returned before original reader", err)
	default:
	}
	if other, err := OpenRetainedStore(config); err == nil {
		other.Close()
		t.Fatal("held original reader lost exclusive lease")
	}
	if err := h.Check(); !errors.Is(err, errStoreInspectionClosed) {
		t.Fatal("late Check entered closed inspection", err)
	}
	unblock()
	if err := <-checked; !errors.Is(err, errStoreInspectionClosed) {
		t.Fatal("held original read accepted after Close", err)
	}
	first := <-closed
	if h.Close() != first {
		t.Fatal("copied Close lost terminal result")
	}
}

func TestDescriptorStoreInspectionNativePhysicalCloseFailureRemainsRetained(t *testing.T) {
	config := descriptorInspectionNativeEmpty(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second*10)
	defer cancel()
	h, err := OpenStoreInspection(ctx, config, 988, 988)
	if err != nil {
		t.Fatal(err)
	}
	h.original.observe = func(stage string, file *os.File) {
		if stage == "before-close" {
			if err := file.Close(); err != nil {
				t.Error(err)
			}
		}
	}
	first := h.Close()
	if first == nil || h.Close() != first {
		t.Fatal("actual late descriptor Close failure erased", first)
	}
}
