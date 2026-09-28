//go:build linux

// Package publication owns the Endpoint duty context's publication refresh
// scheduler mechanism: the single in-flight refresh identity, its wake-up,
// cancellation, joined terminal result, and the fixed-stage failure wrapper.
//
// The scheduler is pure mechanism. It retains no registration or pair state
// and calls back into no duty context; the surrounding endpoint supplies the
// rotation callback to Start and observes the flight through the exported
// Refresh fields. The registration pair lifecycle and the Introduction
// registration entity it schedules are bidirectionally coupled and remain with
// the endpoint root (moving with the introduction subsystem), so this package
// deliberately does not reference them.
package publication

import (
	"context"
	"errors"
	"sync"
	"time"
)

// RefreshContentionRetryDelay bounds the retry pause after a transient Source
// role-member conflict read, before the scheduler attempts rotation again.
const RefreshContentionRetryDelay = 100 * time.Millisecond

// RefreshLifecycle is the sole owner of scheduler identity, wake-up,
// cancellation and its joined terminal result. The surrounding duty context
// supplies the rotation callback but never publishes or replaces a flight
// directly.
type RefreshLifecycle struct {
	mu      sync.Mutex
	flight  *Refresh
	stopped bool
}

// Refresh is one in-flight refresh scheduler run. Context, Wake and Done are
// exported for the endpoint rotation callback and its tests; cancel and err
// stay private to the lifecycle that owns them.
type Refresh struct {
	Context context.Context
	cancel  context.CancelFunc
	Done    chan struct{}
	Wake    chan struct{}
	err     error
}

// RefreshRetirement joins a cancelled scheduler flight during duty shutdown.
type RefreshRetirement struct {
	owner  *RefreshLifecycle
	flight *Refresh
}

// Start begins one scheduler flight after a verified publication
// acknowledgement, or returns the retained flight if one is already live or
// the lifecycle has stopped. Exact retries never move the original refresh
// time or renew the signed registration lifetime.
func (lifecycle *RefreshLifecycle) Start(parent context.Context, run func(*Refresh)) *Refresh {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if lifecycle.stopped {
		return lifecycle.flight
	}
	if lifecycle.flight != nil {
		select {
		case <-lifecycle.flight.Done:
			lifecycle.flight = nil
		default:
			lifecycle.wakeLocked()
			return lifecycle.flight
		}
	}
	ctx, cancel := context.WithCancel(parent)
	flight := &Refresh{Context: ctx, cancel: cancel, Done: make(chan struct{}), Wake: make(chan struct{}, 1)}
	lifecycle.flight = flight
	lifecycle.wakeLocked()
	go func() {
		run(flight)
		lifecycle.mu.Lock()
		if lifecycle.flight == flight {
			lifecycle.stopped = true
		}
		lifecycle.mu.Unlock()
		close(flight.Done)
	}()
	return flight
}

// Wake signals the live scheduler flight, if any, to re-evaluate its schedule.
func (lifecycle *RefreshLifecycle) Wake() {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	lifecycle.wakeLocked()
}

func (lifecycle *RefreshLifecycle) wakeLocked() {
	if lifecycle.flight == nil || lifecycle.flight.Context.Err() != nil {
		return
	}
	select {
	case lifecycle.flight.Wake <- struct{}{}:
	default:
	}
}

// Current returns the live scheduler flight, if any.
func (lifecycle *RefreshLifecycle) Current() *Refresh {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	return lifecycle.flight
}

// MatchesContext reports whether the live scheduler flight runs on ctx.
func (lifecycle *RefreshLifecycle) MatchesContext(ctx context.Context) bool {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	return lifecycle.flight != nil && lifecycle.flight.Context == ctx
}

// Cancel stops the lifecycle and cancels the live flight, returning it.
func (lifecycle *RefreshLifecycle) Cancel() *Refresh {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	lifecycle.stopped = true
	if lifecycle.flight != nil {
		lifecycle.flight.cancel()
	}
	return lifecycle.flight
}

// Join blocks until the flight ends and returns its terminal outcome.
func (lifecycle *RefreshLifecycle) Join(flight *Refresh) error {
	if flight == nil {
		return nil
	}
	<-flight.Done
	return lifecycle.Outcome(flight)
}

// Outcome returns the flight's terminal failure without blocking.
func (lifecycle *RefreshLifecycle) Outcome(flight *Refresh) error {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if flight == nil {
		return nil
	}
	return flight.err
}

// Stop cancels the lifecycle and joins its terminal flight in one call.
func (lifecycle *RefreshLifecycle) Stop() error {
	return lifecycle.Join(lifecycle.Cancel())
}

// StopAsync cancels the lifecycle and returns a retirement that joins the
// terminal flight later, so duty shutdown can interleave other cleanup.
func (lifecycle *RefreshLifecycle) StopAsync() *RefreshRetirement {
	return &RefreshRetirement{owner: lifecycle, flight: lifecycle.Cancel()}
}

// Join blocks for the cancelled scheduler flight and returns its outcome.
func (retirement *RefreshRetirement) Join() error {
	if retirement == nil || retirement.owner == nil {
		return nil
	}
	outcome := retirement.owner.Join(retirement.flight)
	retirement.flight = nil
	return outcome
}

// Fail records the terminal failure for the live flight, if it is still current.
func (lifecycle *RefreshLifecycle) Fail(flight *Refresh, err error) bool {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if flight == nil || lifecycle.flight != flight {
		return false
	}
	flight.err = err
	return true
}

// refreshFailure retains a fixed, locally reportable stage while preserving
// the underlying error for the Endpoint's own terminal cleanup semantics.
type refreshFailure struct {
	stage string
	cause error
}

func (failure *refreshFailure) Error() string { return failure.cause.Error() }

func (failure *refreshFailure) Unwrap() error { return failure.cause }

// RefreshFailureAt wraps cause with a fixed reportable stage.
func RefreshFailureAt(stage string, cause error) error {
	return &refreshFailure{stage: stage, cause: cause}
}

// RefreshFailureStage returns the fixed stage of a wrapped refresh failure, or
// the default "rotation" stage when cause carries no explicit classification.
func RefreshFailureStage(cause error) string {
	var failure *refreshFailure
	if errors.As(cause, &failure) && failure.stage != "" {
		return failure.stage
	}
	return "rotation"
}
