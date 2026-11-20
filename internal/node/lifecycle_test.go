package node

import (
	"context"
	"crypto/sha256"
	"errors"
	hostingbudget "github.com/dianabuilds/ardents-network/internal/hosting"
	localroles "github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"sync/atomic"
	"testing"
	"time"
)

func TestPreparedCancellationRetainsTerminalEventFailure(t *testing.T) {
	outputErr := errors.New("terminal event output failed")
	var observed Event
	config := runtimeConfig{Config: Config{Emit: func(_ context.Context, event Event) error {
		observed = event
		return outputErr
	}}, now: func() time.Time { return time.Unix(100, 0).UTC() }}
	machine := stateMachine{current: statePrepared}
	result, err := terminalWithoutDuty(config, &machine, state.NodeDuty{Assignment: "closed_issuer"}, context.Canceled)
	if result.State != stateNames[stateFailed] || observed.State != stateNames[stateFailed] ||
		!errors.Is(err, context.Canceled) || !errors.Is(err, outputErr) {
		t.Fatalf("prepared cancellation = %+v, event %+v, %v", result, observed, err)
	}
}

func TestWithdrawDoesNotPublishSuccessWhenRoleDrainFails(t *testing.T) {
	cleanupErr := errors.New("injected role drain failure")
	var events []Event
	config := runtimeConfig{Config: Config{Emit: func(_ context.Context, event Event) error {
		events = append(events, event)
		return nil
	}}, now: func() time.Time { return time.Unix(100, 0).UTC() }}
	machine := stateMachine{current: stateReady}
	server := &dutyHandle{Stop: func() {}, Drain: func(context.Context) error { return cleanupErr }}
	result, err := withdraw(config, &machine, server, state.NodeDuty{Assignment: "rendezvous"}, "test withdrawal")
	if !errors.Is(err, cleanupErr) || result.State == stateNames[stateWithdrawn] {
		t.Fatalf("withdraw result = %+v, %v", result, err)
	}
	for _, event := range events {
		if event.State == stateNames[stateWithdrawn] {
			t.Fatal("WITHDRAWN was published after unknown role cleanup")
		}
	}
}

func TestRunReportsReadinessLossDuringQuarantine(t *testing.T) {
	fixture := newClosedAdmissionFixture(t)
	config := fixture.config
	config.Quarantine = time.Millisecond
	if admission := assessAdmission(config, fixture.snapshot); admission.kind != admissionReady {
		t.Fatalf("original closed duty was unavailable: %+v", admission)
	}
	config.cleanup = &dutyCleanup{release: func() error { return releaseLocalDuty(config) }}
	config.Current = func() (state.NodeDuty, error) {
		inactive := fixture.snapshot
		inactive.Fresh = false
		return inactive, nil
	}
	config.Emit = func(_ context.Context, event Event) error {
		if event.State == "READY" {
			t.Fatal("stale closed duty announced readiness")
		}
		return nil
	}
	machine := stateMachine{current: stateAbsent}
	result, err := runDuty(t.Context(), config, &machine, fixture.snapshot)
	const reason = "assignment lost readiness during quarantine: closed Route State is unavailable: closed Route snapshot prerequisites are not satisfied"
	if err == nil || result.State != "FAILED" || result.Reason != reason {
		t.Fatalf("quarantine readiness result = %+v, %v", result, err)
	}
}

type hostingLifetimeTestHost struct {
	closed atomic.Int32
	err    error
}

func (host *hostingLifetimeTestHost) Sample(context.Context, time.Duration) (hostingbudget.Sample, error) {
	return hostingbudget.Sample{}, nil
}

func (host *hostingLifetimeTestHost) Reserve(context.Context, hostingbudget.Traffic, hostingbudget.Traffic, time.Time) (hosting.Reservation, error) {
	return nil, nil
}

func (host *hostingLifetimeTestHost) Close() error {
	host.closed.Add(1)
	return host.err
}

func TestWithdrawTransfersHostingCloseToUnjoinedLateChild(t *testing.T) {
	host := &hostingLifetimeTestHost{}
	config := runtimeConfig{Config: Config{Emit: func(context.Context, Event) error { return nil }},
		now: func() time.Time { return time.Unix(100, 0).UTC() }}
	var released atomic.Int32
	config.cleanup = &dutyCleanup{host: hosting.NewLifetime(host), retained: true, release: func() error {
		released.Add(1)
		return nil
	}}
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
	if err := config.cleanup.Close(); err != nil {
		t.Fatalf("transferred Hosting close failed: %v", err)
	}
	if host.closed.Load() != 0 {
		t.Fatal("shared Hosting closed before the late child joined")
	}
	if released.Load() != 0 {
		t.Fatal("local role was released before the late child joined")
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
	if released.Load() != 0 {
		t.Fatal("failed withdrawal released its local role before expiry")
	}
}

func TestWithdrawPublishesSuccessOnlyAfterProcessCleanup(t *testing.T) {
	for _, selected := range []struct {
		name       string
		hostErr    error
		releaseErr error
	}{
		{name: "complete cleanup"},
		{name: "host close", hostErr: errors.New("host close failed")},
		{name: "local role removal", releaseErr: errors.New("role removal failed")},
	} {
		t.Run(selected.name, func(t *testing.T) {
			host := &hostingLifetimeTestHost{err: selected.hostErr}
			var events []Event
			var released bool
			config := runtimeConfig{Config: Config{Emit: func(_ context.Context, event Event) error {
				if event.State == "WITHDRAWN" && (host.closed.Load() != 1 || !released) {
					t.Error("WITHDRAWN was emitted before Hosting close and local role removal")
				}
				events = append(events, event)
				return nil
			}}, now: func() time.Time { return time.Unix(100, 0).UTC() }}
			config.cleanup = &dutyCleanup{host: hosting.NewLifetime(host), retained: true, release: func() error {
				released = true
				return selected.releaseErr
			}}
			machine := stateMachine{current: stateReady}
			server := &dutyHandle{Stop: func() {}, Drain: func(context.Context) error { return nil }}
			result, err := withdraw(config, &machine, server, state.NodeDuty{Assignment: "rendezvous"}, "test withdrawal")
			want := selected.hostErr
			if want == nil {
				want = selected.releaseErr
			}
			if want == nil && (result.State != "WITHDRAWN" || err != nil) ||
				want != nil && (result.State != "FAILED" || !errors.Is(err, want)) {
				t.Fatalf("withdraw result = %+v, %v", result, err)
			}
			if host.closed.Load() != 1 || released != (selected.hostErr == nil) {
				t.Fatalf("cleanup order: host closes = %d, role released = %v", host.closed.Load(), released)
			}
			for _, event := range events {
				if want != nil && event.State == "WITHDRAWN" {
					t.Fatal("WITHDRAWN published before complete process cleanup")
				}
			}
		})
	}
}

func TestWithdrawRejectsUnjoinedDrainResult(t *testing.T) {
	host := &hostingLifetimeTestHost{}
	joined := make(chan struct{})
	var released bool
	var events []Event
	config := runtimeConfig{Config: Config{Emit: func(_ context.Context, event Event) error {
		events = append(events, event)
		return nil
	}}, now: func() time.Time { return time.Unix(100, 0).UTC() }}
	config.cleanup = &dutyCleanup{host: hosting.NewLifetime(host), retained: true, release: func() error {
		released = true
		return nil
	}}
	machine := stateMachine{current: stateReady}
	server := &dutyHandle{Joined: joined, Stop: func() {}, Drain: func(context.Context) error { return nil }}
	result, err := withdraw(config, &machine, server, state.NodeDuty{Assignment: "rendezvous"}, "test withdrawal")
	if result.State != "FAILED" || err == nil {
		t.Fatalf("unjoined drain result = %+v, %v", result, err)
	}
	if err := config.cleanup.Close(); err != nil || released || host.closed.Load() != 0 {
		t.Fatalf("unjoined resources were released: %v, %v, %d", err, released, host.closed.Load())
	}
	for _, event := range events {
		if event.State == "WITHDRAWN" {
			t.Fatal("unjoined role was reported withdrawn")
		}
	}
	close(joined)
	deadline := time.After(time.Second)
	for host.closed.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("shared Hosting did not close after the unjoined role finished")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func TestUnjoinedRoleKeepsDurableLocalConflict(t *testing.T) {
	fixture := newClosedAdmissionFixture(t)
	config := fixture.config
	config.now = time.Now
	if err := retainLocalDuty(config, fixture.snapshot, "live"); err != nil {
		t.Fatal(err)
	}
	joined := make(chan struct{})
	config.cleanup = &dutyCleanup{retained: true, release: func() error { return releaseLocalDuty(config) }}
	config.cleanup.deferUntil(joined)
	if err := config.cleanup.Close(); err != nil {
		t.Fatal(err)
	}
	roles, err := localroles.Open(localroles.Config{Root: config.LocalRoleStateRoot, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	conflict, checkErr := roles.Conflict(fixture.snapshot.NodeID, sha256.Sum256([]byte(fixture.snapshot.DeclaredFamily)))
	closeErr := roles.Close()
	if checkErr != nil || closeErr != nil || !conflict {
		t.Fatalf("local duty after unknown join = %v, check %v, close %v", conflict, checkErr, closeErr)
	}
	close(joined)
}
