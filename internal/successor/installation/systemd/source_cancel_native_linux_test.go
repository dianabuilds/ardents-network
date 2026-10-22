//go:build network_signing_native

package systemd

import (
	"context"
	"os"
	"testing"
	"time"
)

// Actual private manager mechanism only; no Source Start/Stop or startup authority.
func TestSourceCanceledObservationCloseNative(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-source-startup-native-stand")
	if os.Geteuid() != 0 || err != nil || string(marker) != "ardents-source-startup-native-v1\n" {
		t.Fatal("invalid private Source startup native environment", err)
	}
	for index := 0; index < 32; index++ {
		ctx, cancel := context.WithCancel(t.Context())
		control, err := OpenSourceControl(ctx)
		if err != nil {
			cancel()
			if control != nil {
				_ = control.Close()
			}
			t.Fatal(err)
		}
		reference := control.original.reference
		if err := reference.local(); err != nil {
			cancel()
			_ = control.Close()
			t.Fatal("original local admission failed before cancellation", err)
		}
		// Retain a real observation use blocked behind its original I/O owner.
		reference.ioMu.Lock()
		if err = reference.begin(); err != nil {
			reference.ioMu.Unlock()
			cancel()
			_ = control.Close()
			t.Fatal(err)
		}
		entered := make(chan struct{})
		observed := make(chan error, 1)
		go func() {
			close(entered)
			reference.ioMu.Lock()
			defer reference.ioMu.Unlock()
			defer reference.uses.Done()
			observed <- reference.sourceUnavailable()
		}()
		<-entered
		cancel()
		closed := make(chan error, 1)
		go func() { closed <- control.Close() }()
		end := time.Now().Add(time.Second)
		for !reference.closed.Load() || reference.client.bus.Connected() {
			if time.Now().After(end) {
				reference.ioMu.Unlock()
				<-observed
				<-closed
				t.Fatal("actual Close did not deny and disconnect before join")
			}
			time.Sleep(time.Millisecond)
		}
		reference.ioMu.Unlock()
		observation, cleanup := <-observed, <-closed
		if !sourceReferenceCancellationOnly(observation) || !sourceReferenceCancellationOnly(cleanup) {
			t.Fatalf("original canceled observation invented a close refusal: observation=%v cleanup=%v", observation, cleanup)
		}
	}
	// Exercise complete actual manager calls across the same cancel/Close race,
	// in addition to the deterministic admitted-use ordering control above.
	for range 64 {
		ctx, cancel := context.WithCancel(t.Context())
		control, err := OpenSourceControl(ctx)
		if err != nil {
			cancel()
			if control != nil {
				_ = control.Close()
			}
			t.Fatal(err)
		}
		ready, observed := make(chan struct{}), make(chan error, 1)
		go func() {
			_, err := control.Observe()
			close(ready)
			for err == nil {
				_, err = control.Observe()
			}
			observed <- err
		}()
		<-ready
		cancel()
		cleanup := control.Close()
		observation := <-observed
		if !sourceReferenceCancellationOnly(observation) || !sourceReferenceCancellationOnly(cleanup) {
			t.Fatalf("actual canceled manager calls invented a failure: observation=%v cleanup=%v", observation, cleanup)
		}
	}
}
