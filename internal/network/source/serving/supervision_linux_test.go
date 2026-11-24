package serving

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// These barriers model only the supervisor's physical join contract. They do
// not supply successful State authority, a listener, a process or manager facts.
type sourceRetirementBarriers struct {
	waitRelease, closeRelease chan struct{}
	waitErr, closeErr         error
}

func (*sourceRetirementBarriers) Current() (state.Snapshot, error) {
	return state.Snapshot{}, context.Canceled
}
func (b *sourceRetirementBarriers) Wait(context.Context) error { <-b.waitRelease; return b.waitErr }
func (b *sourceRetirementBarriers) Close() error               { <-b.closeRelease; return b.closeErr }

func TestSourceCancellationJoinsWaitCloseAndInputsBeforeCompletion(t *testing.T) {
	for _, failure := range []string{"none", "wait", "close", "inputs"} {
		t.Run(failure, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				late := errors.New("late original join failure")
				owner := &sourceRetirementBarriers{waitRelease: make(chan struct{}), closeRelease: make(chan struct{})}
				if failure == "wait" {
					owner.waitErr = late
				}
				if failure == "close" {
					owner.closeErr = late
				}
				inputStarted, inputRelease := make(chan struct{}), make(chan struct{})
				joined := make(chan error, 1)
				go func() {
					joined <- Supervise(ctx, owner, ctx.Err, func() {}, ignoreSourceEvent, func() error {
						close(inputStarted)
						<-inputRelease
						if failure == "inputs" {
							return late
						}
						return nil
					})
				}()
				synctest.Wait()
				select {
				case <-joined:
					t.Fatal("cancellation substituted for join")
				default:
				}
				close(owner.closeRelease)
				synctest.Wait()
				select {
				case <-inputStarted:
					t.Fatal("Close alone released inputs before Wait joined")
				default:
				}
				close(owner.waitRelease)
				synctest.Wait()
				select {
				case <-inputStarted:
				default:
					t.Fatal("joined State did not begin input cleanup")
				}
				select {
				case <-joined:
					t.Fatal("unjoined inputs permitted completion")
				default:
				}
				close(inputRelease)
				err := <-joined
				if failure == "none" && err != nil || failure != "none" && !errors.Is(err, late) {
					t.Fatal("original completion outcome lost", err)
				}
			})
		})
	}
}

// Failure fixtures isolate supervisor ordering, never successful State authority.
type failedSourceLifetime struct {
	waitErr, closeErr error
	closed            bool
}

type blockedSourceClose struct {
	failedSourceLifetime
	entered, release chan struct{}
}

func (owner *blockedSourceClose) Close() error {
	close(owner.entered)
	<-owner.release
	return owner.failedSourceLifetime.Close()
}

func TestSourceUnavailableJoinReportsWithoutReturningOriginalCustody(t *testing.T) {
	operation, cleanup := errors.New("original refusal"), errors.New("late physical close")
	owner := &blockedSourceClose{failedSourceLifetime: failedSourceLifetime{closeErr: cleanup}, entered: make(chan struct{}), release: make(chan struct{})}
	notice := make(chan struct{})
	admissionCtx, deny := context.WithCancel(t.Context())
	defer deny()
	joined := make(chan error, 1)
	go func() {
		joined <- Supervise(t.Context(), owner, func() error { return operation }, deny, func(_ context.Context, event Event) error {
			if event.Phase == "retiring" && event.Outcome == "completion-unavailable" {
				close(notice)
			}
			return nil
		}, nil)
	}()
	<-owner.entered
	if admissionCtx.Err() != context.Canceled {
		close(owner.release)
		<-joined
		t.Fatal("physical join began before synchronous admission denial")
	}
	<-notice
	select {
	case <-joined:
		t.Fatal("notice or timeout substituted for original join")
	default:
	}
	close(owner.release)
	if err := <-joined; !errors.Is(err, operation) || !errors.Is(err, cleanup) {
		t.Fatal("late join erased original failures", err)
	}
}

func (f *failedSourceLifetime) Current() (state.Snapshot, error) { return state.Snapshot{}, nil }
func (f *failedSourceLifetime) Wait(ctx context.Context) error {
	if f.waitErr != nil {
		return f.waitErr
	}
	<-ctx.Done()
	return nil
}
func (f *failedSourceLifetime) Close() error { f.closed = true; return f.closeErr }

func TestSourceSupervisionRetainsOperationAndCleanupFailures(t *testing.T) {
	operation, cleanup := errors.New("operation"), errors.New("independent cleanup")
	for _, mode := range []string{"output", "wait", "protection"} {
		t.Run(mode, func(t *testing.T) {
			owner := &failedSourceLifetime{closeErr: cleanup}
			report := ignoreSourceEvent
			check := func() error { return nil }
			switch mode {
			case "output":
				report = func(context.Context, Event) error { return operation }
			case "wait":
				owner.waitErr = operation
			case "protection":
				check = func() error { return operation }
			}
			err := Supervise(t.Context(), owner, check, func() {}, report, nil)
			if !owner.closed || !errors.Is(err, operation) || !errors.Is(err, cleanup) {
				t.Fatal("original users or independent cause lost", owner.closed, err)
			}
		})
	}
}

func TestSourceOutputFailureRetainsOriginalLatchedProcessFailure(t *testing.T) {
	output, process := errors.New("original output failure"), errors.New("original process failure already retained")
	latched := false
	check := func() error {
		if latched {
			return process
		}
		return nil
	}
	owner := &failedSourceLifetime{}
	err := Supervise(t.Context(), owner, check, func() {}, func(context.Context, Event) error {
		latched = true
		return output
	}, nil)
	if !errors.Is(err, output) || !errors.Is(err, process) || !owner.closed {
		t.Fatal("an original owner failure was dropped", err)
	}
}

func TestSourceInputLossAfterHealthyObservationRetainsInputCategory(t *testing.T) {
	// This fixture isolates first-cause projection, not successful State or
	// native process authority. The third observation proves initial readiness
	// was consumed before cancellation; its held late result is deliberately nil.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var checks atomic.Int32
	var lost atomic.Bool
	entered, release := make(chan struct{}), make(chan struct{})
	check := func() error {
		if checks.Add(1) == 3 {
			close(entered)
			<-release
			return nil
		}
		if lost.Load() {
			return artifacts.ErrObservedInput
		}
		return nil
	}
	owner := &failedSourceLifetime{}
	joined := make(chan error, 1)
	go func() { joined <- Supervise(ctx, owner, check, func() {}, ignoreSourceEvent, nil) }()
	<-entered
	lost.Store(true)
	cancel()
	close(release)
	err := <-joined
	var terminal *Terminal
	if !errors.Is(err, artifacts.ErrObservedInput) || !errors.As(err, &terminal) || terminal.FirstOwner != "input" || !owner.closed {
		t.Fatal("last healthy process observation replaced original input refusal", err)
	}
}

func ignoreSourceEvent(context.Context, Event) error { return nil }
