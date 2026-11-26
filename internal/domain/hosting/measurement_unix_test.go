//go:build linux || darwin

package hosting

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/measurement"
	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
)

func TestMeasuredHostingWaitCancellationAndConfirmedResources(t *testing.T) {
	capture, err := measurement.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer capture.Stop()
	root, reading, now := hostingFixture(t)
	owner := openHostingFixture(t, root, reading, now)
	quantity := func(id runtimeevent.Instrument) runtimeevent.Measurement {
		t.Helper()
		capture.Sample()
		spec, _ := id.Spec()
		for _, v := range capture.Latest().Values {
			if v.Name == spec.Name {
				return v
			}
		}
		return runtimeevent.Measurement{}
	}
	if v := quantity(runtimeevent.HostingObservedUsed); !v.Available || v.Current != 100 {
		t.Fatal("confirmed durable quantity absent", v)
	}
	<-owner.gate
	gateHeld := true
	defer func() {
		if gateHeld {
			owner.gate <- struct{}{}
		}
	}()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := owner.Observe(ctx); done <- err }()
	deadline := time.Now().Add(3 * time.Second)
	for quantity(runtimeevent.HostingWaiters).Current != 1 {
		if time.Now().After(deadline) {
			owner.gate <- struct{}{}
			gateHeld = false
			t.Fatal("wait never observed")
		}
		runtime.Gosched()
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	owner.gate <- struct{}{}
	gateHeld = false
	if v := quantity(runtimeevent.HostingWaiters); !v.Available || v.Current != 0 || v.Peak != 1 {
		t.Fatal("canceled waiter retained", v)
	}
	if v := quantity(runtimeevent.HostingObservedUsed); v.Available {
		t.Fatal("failed last observation presented current resources", v)
	}
	if _, err := owner.Observe(t.Context()); err != nil {
		t.Fatal(err)
	}
	if v := quantity(runtimeevent.HostingObservedUsed); !v.Available || v.Current != 100 {
		t.Fatal("confirmed recovery absent", v)
	}
}
