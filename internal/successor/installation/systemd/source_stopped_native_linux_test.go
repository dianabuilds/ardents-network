//go:build linux && network_signing_native

package systemd

import (
	"context"
	"errors"
	"net"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func sourceStoppedNativeStand(t *testing.T) {
	t.Helper()
	marker, e := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	container, ce := os.ReadFile("/run/systemd/container")
	if e != nil || ce != nil || string(marker) != "ardents-source-stopped-native-v1\n" || strings.TrimSpace(string(container)) != "ardents-stopped" {
		t.Fatal("invalid environment: original private stopped Source manager required", e, ce)
	}
}

func TestSourceStoppedNativeIdleOriginalManagerAndCopiedSingleReload(t *testing.T) {
	sourceStoppedNativeStand(t)
	s, e := OpenSourceStopped(t.Context())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	copy := *s
	// This wait specifically falsifies the old real socket's 3-second idle
	// deadline. It is not used as evidence of join or physical completion.
	time.Sleep(3100 * time.Millisecond)
	if e = copy.CheckOriginal(); e != nil {
		t.Fatal("original manager died between finite observations", e)
	}
	if _, e = s.CheckAbsent(); e != nil {
		t.Fatal(e)
	}
	if e = s.Reload(); e != nil {
		t.Fatal("actual one daemon reload", e)
	}
	if e = copy.Reload(); e == nil {
		t.Fatal("copied handle repeated reload")
	}
	if e = s.CheckOriginal(); e == nil {
		t.Fatal("copied refusal was not retained")
	}
	if s.Close() == nil || copy.Close() == nil {
		t.Fatal("copied Close erased original refusal")
	}
	if _, e = os.Lstat("/etc/systemd/system/ardents-source.service"); !os.IsNotExist(e) {
		t.Fatal("manager leaf installed or started Source", e)
	}
}

func TestSourceStoppedNativeCancellationJoinsHeldOriginalObservation(t *testing.T) {
	sourceStoppedNativeStand(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s, e := OpenSourceStopped(ctx)
	if e != nil {
		t.Fatal(e)
	}
	copy := *s
	entered, release := make(chan struct{}), make(chan struct{})
	s.original.reference.owner.observe = func() { close(entered); <-release }
	checked := make(chan error, 1)
	go func() { _, e := s.CheckAbsent(); checked <- e }()
	<-entered
	cancel()
	if e = copy.CheckOriginal(); !errors.Is(e, context.Canceled) {
		t.Fatal("original cancellation did not deny new effects", e)
	}
	// Independently fail the actual original socket Close while the final
	// original observer remains held. Both causes must survive physical join.
	if e = s.original.reference.owner.client.transport.socket.Close(); e != nil {
		t.Fatal(e)
	}
	closed := make(chan error, 1)
	go func() { closed <- copy.Close() }()
	for !s.original.reference.owner.closed.Load() {
		runtime.Gosched()
	}
	select {
	case <-closed:
		t.Fatal("Close escaped before original observer joined")
	default:
	}
	close(release)
	if e = <-checked; !errors.Is(e, context.Canceled) {
		t.Fatal("late observed success", e)
	}
	first := <-closed
	if !errors.Is(first, context.Canceled) || !errors.Is(first, net.ErrClosed) {
		t.Fatal("original/physical causes lost", first)
	}
	if !errors.Is(s.Close(), net.ErrClosed) || copy.CheckOriginal() == nil {
		t.Fatal("copy/late use renewed original manager")
	}
}

func TestSourceStoppedNativeIdleCancellationAndActualManagerLoss(t *testing.T) {
	sourceStoppedNativeStand(t)
	ctx, cancel := context.WithCancel(t.Context())
	s, e := OpenSourceStopped(ctx)
	if e != nil {
		t.Fatal(e)
	}
	copy := *s
	cancel()
	if e = s.Close(); !errors.Is(e, context.Canceled) {
		t.Fatal("idle original cancellation disappeared", e)
	}
	if !errors.Is(copy.Close(), context.Canceled) {
		t.Fatal("copied close changed outcome")
	}
	s, e = OpenSourceStopped(t.Context())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = s.original.reference.owner.client.transport.socket.Close(); e != nil {
		t.Fatal(e)
	}
	if e = s.CheckOriginal(); e == nil {
		t.Fatal("actual disconnected original manager accepted")
	}
	if e = s.Reload(); e == nil {
		t.Fatal("manager loss reconnected for mutation")
	}
}
