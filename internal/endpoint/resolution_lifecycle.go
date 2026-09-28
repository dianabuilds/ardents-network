//go:build linux

package endpoint

import (
	"context"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/endpoint/source"
)

// resolutionLifecycle owns the one lookup-or-publication flight under
// dutyContext.mu. Stop revokes it without clearing its identity: retirement
// still joins the exact flight after Source prefix and issuance teardown.
type resolutionLifecycle struct {
	current *resolutionFlight
	stopped bool
}

// resolutionFlight retains the Source acquired at admission until completion.
// Its caller callback is joined before the acquisition, slot and done signal
// are released, so a cancelled caller cannot act through a later flight.
type resolutionFlight struct {
	context       context.Context
	cancel        context.CancelFunc
	done          chan struct{}
	source        *source.ResolutionAcquisition
	receiver      [32]byte
	stopCaller    func() bool
	callerStopped <-chan struct{}
	callerJoin    sync.Once
	finished      bool
}

func (lifecycle *resolutionLifecycle) BusyLocked() bool {
	return lifecycle.current != nil
}

func (lifecycle *resolutionLifecycle) CurrentLocked(flight *resolutionFlight) bool {
	return flight != nil && !lifecycle.stopped && lifecycle.current == flight
}

func (lifecycle *resolutionLifecycle) CurrentSourceLocked(flight *resolutionFlight, sourceOwner *source.Lifecycle) bool {
	return lifecycle.CurrentLocked(flight) && flight.source != nil && flight.source.CurrentLocked(sourceOwner)
}

// BeginLocked takes the exact admitted Source acquisition. A failed admission
// releases the unused acquisition; it cannot replace an active flight.
func (lifecycle *resolutionLifecycle) BeginLocked(parent, caller context.Context, acquisition *source.ResolutionAcquisition) *resolutionFlight {
	if lifecycle.stopped || lifecycle.current != nil || parent == nil || caller == nil || acquisition == nil {
		if acquisition != nil {
			acquisition.Release()
		}
		return nil
	}
	attempt, cancel := context.WithCancel(parent)
	interrupted := make(chan struct{})
	stop := context.AfterFunc(caller, func() { defer close(interrupted); cancel() })
	flight := &resolutionFlight{context: attempt, cancel: cancel, done: make(chan struct{}), source: acquisition,
		stopCaller: stop, callerStopped: interrupted}
	lifecycle.current = flight
	return flight
}

// JoinCaller cancels the flight and waits for a concurrent caller-cancellation
// callback before terminal state can be published under dutyContext.mu.
func (lifecycle *resolutionLifecycle) JoinCaller(flight *resolutionFlight) {
	if flight == nil {
		return
	}
	flight.cancel()
	flight.callerJoin.Do(func() {
		if flight.stopCaller != nil && !flight.stopCaller() {
			<-flight.callerStopped
		}
	})
}

// StopLocked revokes admission and the current flight before retirement joins
// any child. It leaves the flight installed until its own completion.
func (lifecycle *resolutionLifecycle) StopLocked() *resolutionFlight {
	lifecycle.stopped = true
	flight := lifecycle.current
	if flight != nil {
		flight.cancel()
	}
	return flight
}

// FinishLocked releases the acquisition exactly once, lets the Context retain
// an actual cleanup failure, then clears the slot and closes the join barrier.
// The caller holds dutyContext.mu and has already joined caller cancellation.
func (lifecycle *resolutionLifecycle) FinishLocked(flight *resolutionFlight, retainFailure func()) {
	if flight == nil || flight.finished {
		return
	}
	if flight.source != nil {
		flight.source.Release()
		flight.source = nil
	}
	if retainFailure != nil {
		retainFailure()
	}
	if lifecycle.current == flight {
		lifecycle.current = nil
	}
	flight.finished = true
	close(flight.done)
}

func (flight *resolutionFlight) Join() {
	if flight != nil {
		<-flight.done
	}
}
