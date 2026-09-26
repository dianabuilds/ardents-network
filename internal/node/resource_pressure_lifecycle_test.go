package node

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

func TestResourcePressureDeadlineHasSafeLifecycleReason(t *testing.T) {
	fixture := newLifecycleFixture(t)
	events := make(chan Event, 32)
	fixture.config.Current = func() (state.NodeDuty, error) { return fixture.snapshot, nil }
	fixture.config.Emit = func(_ context.Context, event Event) error { events <- event; return nil }
	fixture.config.ResourceProfile = "h3-np1-v1"
	var expired atomic.Bool
	fixture.config.ResourceMeasure = func() (resource.Sample, error) {
		if expired.Load() {
			return resource.Sample{}, context.DeadlineExceeded
		}
		return resource.Sample{}, nil
	}
	type outcome struct {
		result Result
		err    error
	}
	finished := make(chan outcome, 1)
	go func() {
		result, err := Run(context.Background(), fixture.config)
		finished <- outcome{result: result, err: err}
	}()
	waitForState(t, events, "READY")
	expired.Store(true)
	failed := waitForStateEvent(t, events, "FAILED")
	const reason = "resource pressure sampling timed out"
	if failed.Reason != reason {
		t.Fatalf("FAILED reason = %q, want %q", failed.Reason, reason)
	}
	select {
	case result := <-finished:
		if result.result.State != "FAILED" || result.result.Reason != reason || !errors.Is(result.err, context.DeadlineExceeded) {
			t.Fatalf("result = %+v, error = %v", result.result, result.err)
		}
	case <-time.After(testLifecycleWait):
		t.Fatal("Node did not join after resource sampling timeout")
	}
}
