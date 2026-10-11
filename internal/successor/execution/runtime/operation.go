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
	publisher          bool
	administration     AdministrationContext
}

func (*Operation) localPermission() {}

func (operation *Operation) Context() context.Context { return operation.ctx }

func (operation *Operation) Check() error {
	if operation == nil || operation.job == nil || operation.lease == nil || operation.ctx == nil || operation.caller == nil {
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
	if operation == nil || operation.cancel == nil {
		return
	}
	operation.once.Do(func() {
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
	if operation == nil || err == nil {
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
