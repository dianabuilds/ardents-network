//go:build linux

package node

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestClosedNodeFailsBeforeReadinessOnKeyMismatch(t *testing.T) {
	fixture := newClosedAdmissionFixture(t)
	fixture.config.ClosedForwarding.HostingRoot = hostingFixtureRoot(t)
	fixture.snapshot.NodePublicKey[0]++
	fixture.config.Current = func() (state.NodeDuty, error) { return fixture.snapshot, nil }
	fixture.config.Emit = func(_ context.Context, event Event) error {
		if event.State == "READY" {
			t.Fatal("key-mismatched duty announced readiness")
		}
		return nil
	}
	result, err := Run(t.Context(), fixture.config.Config)
	if err == nil || result.State != "FAILED" || result.Reason != "local Node identity or key does not match verified state" {
		t.Fatalf("key mismatch result = %+v, %v", result, err)
	}
}

func TestPreparedClosedNodeFailsWhenRecordDisappears(t *testing.T) {
	fixture := newClosedAdmissionFixture(t)
	fixture.config.ClosedForwarding.HostingRoot = hostingFixtureRoot(t)
	fixture.snapshot.Fresh = false
	var removed atomic.Bool
	fixture.config.Current = func() (state.NodeDuty, error) {
		snapshot := fixture.snapshot
		snapshot.RecordPresent = !removed.Load()
		return snapshot, nil
	}
	events := make(chan Event, 8)
	fixture.config.Emit = func(_ context.Context, event Event) error {
		events <- event
		return nil
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	type outcome struct {
		result Result
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := Run(ctx, fixture.config.Config)
		done <- outcome{result, err}
	}()
	waitForStateEvent(t, events, "PREPARED")
	removed.Store(true)
	select {
	case got := <-done:
		if got.err == nil || got.result.State != "FAILED" || got.result.Reason != "local Node has no accepted materialized record" {
			t.Fatalf("removed record result = %+v, %v", got.result, got.err)
		}
	case <-ctx.Done():
		t.Fatal("prepared Node did not terminate after record removal")
	}
}
