//go:build linux || darwin

package serving

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// sourceLifetime is the exact original State contract used by supervision.
// It grants no policy callback or replacement State construction.
type sourceLifetime interface {
	Current() (state.Snapshot, error)
	Wait(context.Context) error
	Close() error
}

type sourceObservation struct {
	owner string
	err   error
}

// Supervise retains one original State, its observations and input joins.
func Supervise(ctx context.Context, owner sourceLifetime, check func() error, deny context.CancelFunc, report func(context.Context, Event) error, joinInputs func() error) error {
	// Original cancellation starts the same physical Close even while an
	// observation is held. No callback or result notification is a join.
	closed := make(chan error, 1)
	var closeOnce sync.Once
	beginClose := func() { closeOnce.Do(func() { go func() { closed <- owner.Close() }() }) }
	callbackDone := make(chan struct{})
	stopCallback := context.AfterFunc(ctx, func() { deny(); beginClose(); close(callbackDone) })
	defer func() {
		if !stopCallback() {
			<-callbackDone
		}
	}()
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	observationCtx, stopObservation := context.WithCancel(ctx)
	defer stopObservation()
	waited := make(chan error, 1)
	go func() { waited <- owner.Wait(waitCtx) }()

	// One original observation at a time, retained through retirement. This
	// keeps denial/interrupt/notice independent of physical Check or output I/O.
	observing := make(chan sourceObservation, 1)
	go func() {
		_, err := owner.Current()
		category := "state"
		if errors.Is(err, artifacts.ErrObservedInput) {
			category = "input"
		}
		if err == nil {
			err = check()
			category = sourceCheckOwner(err)
		}
		if err == nil {
			err = report(observationCtx, Event{Phase: "listening", Outcome: "original-source"})
			category = "output"
		}
		if err == nil {
			err = check()
			category = sourceCheckOwner(err)
		}
		observing <- sourceObservation{category, err}
	}()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var first error
	firstOwner := "caller"
	var waitErr error
	for first == nil {
		select {
		case <-ctx.Done():
			firstOwner = "caller"
			goto retire
		case observation := <-observing:
			observing = nil
			first, firstOwner = observation.err, observation.owner
			if first != nil && observation.owner == "state" && ctx.Err() != nil && check() == ctx.Err() {
				first, firstOwner = nil, "caller"
				goto retire
			}
			if first == ctx.Err() && ctx.Err() != nil {
				first, firstOwner = nil, "caller"
				goto retire
			}
		case waitErr = <-waited:
			waited = nil
			firstOwner = "state"
			if waitErr == nil && ctx.Err() != nil {
				firstOwner = "caller"
				goto retire
			}
			first = waitErr
			if first == nil {
				first = errors.New("background work for Source ended")
			}
		case <-ticker.C:
			if observing == nil {
				observing = make(chan sourceObservation, 1)
				original := observing
				go func() {
					err := check()
					original <- sourceObservation{sourceCheckOwner(err), err}
				}()
			}
		}
	}
retire:
	deny()
	stopObservation()
	cancel()
	beginClose()
	var closeErr error
	// The failed notice covers *all* outstanding original users, not just
	// Close: held Wait/Check/output still retain every needed input/process pin.
	notice := time.NewTimer(5 * time.Second)
	defer notice.Stop()
	unavailable := func() {
		first = errors.Join(first, errors.New("completion unavailable for Source"), report(context.Background(), Event{Phase: "retiring", Outcome: "completion-unavailable"}))
		if firstOwner == "caller" {
			firstOwner = "completion"
		}
	}
	for closed != nil || waited != nil || observing != nil {
		select {
		case closeErr = <-closed:
			closed = nil
		case waitErr = <-waited:
			waited = nil
		case observation := <-observing:
			observing = nil
			if observation.err != nil && observation.err != ctx.Err() && observation.err != observationCtx.Err() {
				// Our own cancellation can close State during Current. Its
				// real terminal failures are retained by the same Close/Wait.
				if !(observation.owner == "state" && ctx.Err() != nil && check() == ctx.Err()) {
					if first == nil {
						firstOwner = observation.owner
					}
					first = errors.Join(first, observation.err)
				}
			}
		case <-notice.C:
			unavailable()
		}
	}
	// This is only physical retirement of command-owned inputs. No accepting
	// callback or State policy crosses here. The same notice spans their native
	// observers and actual descriptor Close; process pins still belong to caller.
	var inputCleanup error
	if joinInputs != nil {
		joinedInputs := make(chan error, 1)
		go func() { joinedInputs <- joinInputs() }()
		for joinedInputs != nil {
			select {
			case inputCleanup = <-joinedInputs:
				joinedInputs = nil
			case <-notice.C:
				unavailable()
			}
		}
	}
	processErr := check()
	if processErr == context.Canceled || processErr == ctx.Err() {
		processErr = nil
	}
	if first == nil && waitErr == nil && closeErr == nil && processErr == nil && inputCleanup == nil {
		return nil
	}
	if first == nil && firstOwner == "caller" {
		switch {
		case closeErr != nil || waitErr != nil:
			firstOwner = "state"
		case inputCleanup != nil:
			firstOwner = "input"
		case processErr != nil:
			firstOwner = sourceCheckOwner(processErr)
		}
	}
	return &Terminal{FirstOwner: firstOwner, First: first, Process: processErr, Wait: waitErr, StateClose: closeErr, InputClose: inputCleanup}
}

func sourceCheckOwner(err error) string {
	if errors.Is(err, artifacts.ErrObservedInput) {
		return "input"
	}
	return "process"
}
