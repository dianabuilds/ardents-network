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

func TestClosedHostingLifetimeDefersCloseUntilJoined(t *testing.T) {
	host := &hostingLifetimeTestHost{}
	lifetime := newClosedHostingLifetime(host)
	joined := make(chan struct{})
	if !lifetime.deferCloseUntil(joined) {
		t.Fatal("did not transfer Hosting close to late join")
	}
	if err := lifetime.close(); err != nil {
		t.Fatalf("deferred close = %v", err)
	}
	if host.closed.Load() != 0 {
		t.Fatalf("Hosting closed before child join: %d", host.closed.Load())
	}
	close(joined)
	deadline := time.After(time.Second)
	for host.closed.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("Hosting did not close after child join")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if host.closed.Load() != 1 {
		t.Fatalf("Hosting close calls = %d", host.closed.Load())
	}
}

type hostingLifetimeTestHost struct{ closed atomic.Int32 }

func (host *hostingLifetimeTestHost) Sample(context.Context, time.Duration) (resource.HostingSample, error) {
	return resource.HostingSample{}, nil
}

func (host *hostingLifetimeTestHost) Reserve(context.Context, resource.HostingTraffic, resource.HostingTraffic, time.Time) (closedForwardingHostReservation, error) {
	return nil, nil
}

func (host *hostingLifetimeTestHost) Close() error {
	host.closed.Add(1)
	return nil
}

func TestWithdrawTransfersHostingCloseToUnjoinedLateChild(t *testing.T) {
	host := &hostingLifetimeTestHost{}
	config := runtimeConfig{Config: Config{Emit: func(context.Context, Event) error { return nil }},
		now:          func() time.Time { return time.Unix(100, 0).UTC() },
		hostLifetime: newClosedHostingLifetime(host)}
	machine := stateMachine{current: stateReady}
	joined := make(chan struct{})
	server := &dutyHandle{Stop: func() {}, Joined: joined,
		Drain: func(context.Context) error { return context.DeadlineExceeded }}
	result, err := withdraw(config, &machine, server, state.NodeDuty{Assignment: "rendezvous"}, "test withdrawal")
	if !errors.Is(err, context.DeadlineExceeded) || result.State == stateNames[stateWithdrawn] {
		t.Fatalf("withdraw result = %+v, %v", result, err)
	}
	// The deferred Run-level close must transfer, not close: a late child can
	// still release its Hosting reservation against the shared handle (F-62).
	if err := config.hostLifetime.close(); err != nil {
		t.Fatalf("transferred Hosting close failed: %v", err)
	}
	if host.closed.Load() != 0 {
		t.Fatal("shared Hosting closed before the late child joined")
	}
	close(joined)
	deadline := time.After(time.Second)
	for host.closed.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("shared Hosting did not close after the late child joined")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}
