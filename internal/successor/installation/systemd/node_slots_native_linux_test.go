//go:build network_signing_native

package systemd

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNodeSlotsNativeIndependentReadOnlyCustodyAndDescriptorReturn(t *testing.T) {
	forwardingManagerStand(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	open := func() {
		t.Helper()
		s, err := OpenNodeSlots(ctx)
		if s != nil {
			defer func() {
				if closeErr := s.Close(); closeErr != nil {
					t.Error("original manager completion", closeErr)
				}
			}()
		}
		if err != nil {
			t.Fatal("genuine five-slot absence", err)
		}
		paths, err := s.CheckAbsent()
		if err != nil || len(paths) == 0 {
			t.Fatal("original manager search paths unavailable", err)
		}
		paths[0] = "/foreign-detached-path"
		if err := s.CheckOriginal(); err != nil {
			t.Fatal("detached copy changed original manager custody", err)
		}
		copy := *s
		if err := copy.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := s.CheckAbsent(); err == nil {
			t.Fatal("copied completion admitted another absence round")
		}
	}
	open() // Warm actual transport/runtime descriptors before native counting.
	before := managerFDCount(t)
	for range 8 {
		open()
		if after := managerFDCount(t); after != before {
			t.Fatal("original descriptors retained after joined Close", before, after)
		}
	}
}

func TestNodeSlotsNativeCloseRetainsOriginalObserverAndCancellation(t *testing.T) {
	forwardingManagerStand(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s, err := OpenNodeSlots(ctx)
	if s != nil {
		defer s.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	s.original.reference.observe = func() {
		once.Do(func() { close(entered); <-release })
	}
	observed, closed := make(chan error, 1), make(chan error, 1)
	go func() { observed <- s.CheckOriginal() }()
	<-entered
	cancel()
	copy := *s
	go func() { closed <- copy.Close() }()
	if err := s.CheckOriginal(); !errors.Is(err, context.Canceled) {
		t.Fatal("original caller loss waited behind the held observer", err)
	}
	select {
	case <-closed:
		t.Fatal("physical observer was not joined")
	default:
	}
	unblock()
	if err := <-observed; !errors.Is(err, context.Canceled) {
		t.Fatal("original cancellation was replaced", err)
	}
	result := <-closed
	if !errors.Is(result, context.Canceled) || s.Close() != result {
		t.Fatal("copies did not retain the same joined result", result)
	}
}
