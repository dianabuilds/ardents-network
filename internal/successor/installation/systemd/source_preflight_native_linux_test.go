//go:build linux && network_signing_native

package systemd

import (
	"context"
	"errors"
	"net"
	"os"
	"runtime"
	"sync"
	"testing"
)

func nativeSourceManager(t *testing.T) *SourcePreflight {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: root native original manager required")
	}
	r, err := OpenSourcePreflight(t.Context())
	if err != nil {
		if r != nil {
			err = errors.Join(err, r.Close())
		}
		t.Fatal("invalid environment: original native manager", err)
	}
	if paths, err := r.Check(); err != nil || len(paths) == 0 {
		t.Fatal("original complete Source namespace", errors.Join(err, r.Close()))
	}
	return r
}

func TestSourcePreflightNativeHeldManagerObservationJoinsBeforeClose(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	r, err := OpenSourcePreflight(ctx)
	if err != nil {
		if r != nil {
			err = errors.Join(err, r.Close())
		}
		t.Fatal(err)
	}
	if _, err = r.Check(); err != nil {
		t.Fatal(errors.Join(err, r.Close()))
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	r.owner.observe = func() { once.Do(func() { close(entered); <-release }) }
	checked := make(chan error, 1)
	go func() { _, err := r.Check(); checked <- err }()
	<-entered
	// Actual manager replies have completed, but their original user is held.
	// Independently lose the real socket before its physical Close is requested.
	if err = r.owner.client.transport.socket.Close(); err != nil {
		t.Fatal(err)
	}
	cancel()
	closed := make(chan error, 1)
	copy := *r
	go func() { closed <- copy.Close() }()
	for {
		if r.owner.closed.Load() {
			break
		}
		runtime.Gosched()
	}
	select {
	case <-closed:
		t.Fatal("native manager Close released before original user joined")
	default:
	}
	select {
	case <-checked:
		t.Fatal("manager observation escaped before original callback joined")
	default:
	}
	close(release)
	if err = <-checked; !errors.Is(err, context.Canceled) {
		t.Fatal("late original result bypassed cancellation", err)
	}
	err = <-closed
	if !errors.Is(err, context.Canceled) || !errors.Is(err, net.ErrClosed) {
		t.Fatal("first cancellation or independent physical Close lost", err)
	}
	if _, err = copy.Check(); err == nil {
		t.Fatal("copied late manager result revived")
	}
}

func TestSourcePreflightNativeLateManagerSocketLoss(t *testing.T) {
	r := nativeSourceManager(t)
	entered, release := make(chan struct{}), make(chan struct{})
	r.owner.observe = func() { close(entered); <-release }
	checked := make(chan error, 1)
	go func() { _, err := r.Check(); checked <- err }()
	<-entered
	if err := r.owner.client.transport.socket.Close(); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-checked; err == nil {
		t.Fatal("late socket loss accepted without caller cancellation")
	}
	if _, err := r.Check(); err == nil {
		t.Fatal("late socket loss revived original manager")
	}
	if err := r.Close(); !errors.Is(err, net.ErrClosed) {
		t.Fatal("independent Close failure lost", err)
	}
}

func TestSourcePreflightNativeOriginalManagerCopiesAndLateUse(t *testing.T) {
	r := nativeSourceManager(t)
	copy := *r
	if _, err := copy.Check(); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := copy.Check(); err == nil {
		t.Fatal("copied manager reconnected after Close")
	}
	if err := copy.Close(); err != nil {
		t.Fatal("repeat Close changed result", err)
	}
}

func TestSourcePreflightNativeOriginalManagerLossAndCloseFailure(t *testing.T) {
	r := nativeSourceManager(t)
	// The real retained original socket is lost. No supplied manager result or
	// simulated successful observation is used; independent late Close remains.
	if err := r.owner.client.transport.socket.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Check(); err == nil {
		t.Fatal("lost original manager accepted")
	}
	if err := r.Close(); !errors.Is(err, net.ErrClosed) {
		t.Fatal("independent original socket Close failure lost", err)
	}
	if _, err := r.Check(); err == nil {
		t.Fatal("manager loss revived")
	}
}

func TestSourcePreflightNativeOriginalManagerIdentityMismatch(t *testing.T) {
	r := nativeSourceManager(t)
	// Corrupt only the retained expected owner after a genuine original round.
	// A new round must observe the original bus owner and reject this mismatch.
	original := r.owner.client.manager
	r.owner.client.manager = ":999999.999999"
	if _, err := r.Check(); err == nil {
		t.Fatal("foreign manager identity accepted")
	}
	r.owner.client.manager = original
	if _, err := r.Check(); err == nil {
		t.Fatal("restored manager facts revived denied lifetime")
	}
	if err := r.Close(); err == nil {
		t.Fatal("first manager failure lost at Close")
	}
}

func TestSourcePreflightNativeOriginalCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	r, err := OpenSourcePreflight(ctx)
	if err != nil {
		if r != nil {
			_ = r.Close()
		}
		t.Fatal(err)
	}
	if _, err = r.Check(); err != nil {
		t.Fatal(err)
	}
	copy := *r
	cancel()
	if _, err = copy.Check(); !errors.Is(err, context.Canceled) {
		t.Fatal("copied handle lost original caller", err)
	}
	if err = r.Close(); !errors.Is(err, context.Canceled) {
		t.Fatal("original cancellation lost", err)
	}
}
