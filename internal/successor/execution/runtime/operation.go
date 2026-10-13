package runtime

import (
	"context"
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
)

// Operation retains one exact qualified Job and its byte-Grant lease. The
// parent composition retains its independently admitted local surface;
// worker permission never supplies Publisher administration or Service rights.
type Operation struct {
	self               *Operation
	invocation         *Invocation
	job                *execution.Job
	lease              *execution.ActiveSession
	ctx                context.Context
	caller             context.Context
	cancel             context.CancelFunc
	stop               func() bool
	callbackDone, done chan struct{}
	once               sync.Once
	resultMu           sync.Mutex
	firstCleanup       error
	contractMu         sync.Mutex
	sendLimit          uint64
	receiveLimit       uint64
	surface            execution.Surface
	connectionContext  [32]byte
	connectionClaimed  bool
	publisher          bool
	administration     AdministrationContext
}

func (*Operation) localPermission() {}

func (operation *Operation) Context() context.Context { return operation.ctx }

func (operation *Operation) Check() error {
	if operation == nil || operation.self != operation || operation.job == nil || operation.lease == nil || operation.ctx == nil || operation.caller == nil {
		return errors.New("execution operation is absent")
	}
	return errors.Join(operation.job.Check(), operation.lease.Context().Err(), operation.ctx.Err(), operation.caller.Err())
}

// CheckPublisher requires the original qualified snapshot worker invocation.
// It is local execution authority, never a Credential or accepting Service.
func (operation *Operation) CheckPublisher() error {
	if operation == nil || !operation.publisher {
		return errors.New("qualified Publisher operation is absent")
	}
	return operation.Check()
}

// Close joins the operation caller’s cancellation callback and completes its
// retained use only after the caller has joined every borrowed resource.
func (operation *Operation) Close() {
	if operation == nil || operation.self != operation || operation.cancel == nil {
		return
	}
	operation.once.Do(func() {
		invocation := operation.invocation
		if invocation != nil {
			invocation.useMu.Lock()
			input := invocation.readerInput
			completed := input != nil && input.completed && invocation.operation == operation
			invocation.useMu.Unlock()
			if completed {
				// Caller-owned borrowers have joined. Allow the still-live
				// original Reader to acknowledge RESULT and join its IPC
				// before ending the authority needed for that final effect.
				close(input.finish)
				<-invocation.observationDone
			}
		}
		operation.cancel()
		if !operation.stop() {
			<-operation.callbackDone
		}
		close(operation.done)
	})
}

// RetainCleanup consumes only a result from physically closing this operation’s
// own borrowers. Failure synchronously denies sibling effects before waiting
// for any remaining resource; nil and later results never erase the first.
func (operation *Operation) RetainCleanup(err error) {
	if operation == nil || operation.self != operation || err == nil {
		return
	}
	operation.resultMu.Lock()
	first := operation.firstCleanup == nil
	if first {
		operation.firstCleanup = err
	}
	operation.resultMu.Unlock()
	if first {
		operation.job.FailCleanup(err)
	}
}

func (operation *Operation) cleanupError() error {
	operation.resultMu.Lock()
	defer operation.resultMu.Unlock()
	return operation.firstCleanup
}
