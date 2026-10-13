package runtime

import (
	"context"
	"crypto/rand"
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
	"github.com/dianabuilds/ardents-network/internal/successor/execution/worker"
)

// Invocation retains the original qualified launch and its one complete use.
// Its local Session remains live through joined-result acceptance; the caller
// subsequently closes the owning generation. Possession grants no Service or
// remote authority, and no worker attachment is exported by this consumer.
type Invocation struct {
	job              *execution.Job
	administration   AdministrationContext
	activation       *worker.Activation
	cleanup          *worker.Cleanup
	grant            *execution.Authority
	lease            *execution.ActiveSession
	stop             func() bool
	callbackDone     chan struct{}
	observationDone  chan struct{}
	observationError error
	useMu            sync.Mutex
	closing          bool
	operationDone    chan struct{}
	operation        *Operation
	done             chan struct{}
	result           error
	cleanupResult    error
	publisher        bool
	publisherInput   *publisherConnectionInput
	readerCaller     context.Context
	readerInput      *readerConnectionInput
	surface          execution.Surface
}

var errUnexpectedWorkerEnd = errors.New("qualified worker attachment ended unexpectedly")

// This bounded setup/Control consumer exports no attachment and permits no
// post-READY Text frames. Its sole reader observes EOF, credential/FD refusal
// or unsolicited bytes; a future Text consumer must own its protocol reader
// instead of sharing this reader. Retirement orders loss against caller close.
func (invocation *Invocation) observeAttachment() {
	invocation.observationDone = make(chan struct{})
	if invocation.publisher {
		invocation.observePublisherAttachment()
		return
	}
	if invocation.readerCaller != nil {
		invocation.observeReaderAttachment()
		return
	}
	go func() {
		defer close(invocation.observationDone)
		var byteBuffer [1]byte
		_, err := invocation.activation.Attachment.Read(byteBuffer[:])
		if invocation.job.Retire() {
			invocation.observationError = errors.Join(errUnexpectedWorkerEnd, err)
		}
	}()
}

func (invocation *Invocation) beginJoin() {
	go func() {
		<-invocation.job.Context().Done()
		invocation.job.Retire()
		invocation.grant.Close()
		invocation.lease.Release()
		if !invocation.stop() {
			<-invocation.callbackDone
		}
		physical := invocation.activation.Attachment.Close()
		invocation.job.FailCleanup(physical)
		<-invocation.observationDone
		invocation.useMu.Lock()
		invocation.closing = true
		operationDone := invocation.operationDone
		operation := invocation.operation
		invocation.useMu.Unlock()
		if operationDone != nil {
			<-operationDone
			physical = errors.Join(physical, operation.cleanupError())
		}
		physical = errors.Join(physical, invocation.cleanup.Close())
		invocation.cleanupResult = invocation.job.Finish(physical)
		invocation.result = errors.Join(invocation.cleanupResult, invocation.observationError)
		close(invocation.done)
	}()
}

func (invocation *Invocation) BeginOperation(ctx context.Context) (*Operation, error) {
	if invocation == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("qualified execution operation is unavailable")
	}
	invocation.useMu.Lock()
	defer invocation.useMu.Unlock()
	if invocation.closing || invocation.operationDone != nil || invocation.lease.Context().Err() != nil {
		return nil, errors.New("qualified execution operation is unavailable or consumed")
	}
	var localContext [32]byte
	if _, err := rand.Read(localContext[:]); err != nil || localContext == [32]byte{} {
		return nil, errors.New("original execution Connection context is unavailable")
	}
	if err := invocation.job.ClaimOperation(); err != nil {
		return nil, err
	}
	bounded, cancel := context.WithCancel(invocation.lease.Context())
	operation := &Operation{invocation: invocation, job: invocation.job, lease: invocation.lease, ctx: bounded, caller: ctx, cancel: cancel,
		callbackDone: make(chan struct{}), done: make(chan struct{}), publisher: invocation.publisher, administration: invocation.administration}
	operation.self = operation
	operation.surface, operation.connectionContext = invocation.surface, localContext
	operation.stop = context.AfterFunc(ctx, func() { defer close(operation.callbackDone); cancel() })
	invocation.operationDone = operation.done
	invocation.operation = operation
	return operation, nil
}

// Close interrupts first, then waits for the caller’s operation and original
// pinned descendants. A timeout or close request never substitutes for join.
func (invocation *Invocation) Close() error {
	if invocation == nil {
		return nil
	}
	invocation.job.Retire()
	<-invocation.done
	return invocation.result
}

func (invocation *Invocation) CompletedCurrent() bool {
	if invocation == nil {
		return false
	}
	cleanup, completed := invocation.CleanupResult()
	return completed && cleanup == nil && invocation.observationError == nil && invocation.job.CompletedCurrent()
}

// CleanupResult observes the retained original physical retirement separately
// from the worker's use failure. False means the original operation, callbacks
// or pinned descendants have not joined; cancellation is not completion.
// Close still returns the complete retained result, including worker failure.
// This observation grants no current Job, preparation or replacement authority.
func (invocation *Invocation) CleanupResult() (error, bool) {
	if invocation == nil {
		return nil, false
	}
	select {
	case <-invocation.done:
		return invocation.cleanupResult, true
	default:
		return nil, false
	}
}

// Launch admits one exact local session before effects and returns only the
// privately bound original invocation, after installed READY reobservation.
func (owner *Owner) Launch(ctx context.Context, principal [32]byte, surface execution.Surface) (_ *Invocation, result error) {
	return owner.launchInvocation(ctx, principal, surface, nil, false, false)
}

func (owner *Owner) launchInvocation(ctx context.Context, principal [32]byte, surface execution.Surface, snapshot []byte, publisher, reader bool) (_ *Invocation, result error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("execution launch is unavailable")
	}
	if reader && (publisher || surface != execution.Connection || len(snapshot) != 0) {
		return nil, errors.New("reader invocation purpose is invalid")
	}
	capability, err := owner.authority.Admit(principal, surface)
	if err != nil {
		return nil, err
	}
	session, err := owner.supervisor.Activate(ctx, capability, principal, surface)
	if err != nil {
		return nil, err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, session.Close())
		}
	}()
	job, err := session.BeginJob()
	if err != nil {
		return nil, err
	}
	var readerCaller context.Context
	if reader {
		readerCaller = ctx
	}
	invocation, err := owner.launchInitializedJob(job, surface, snapshot, publisher, readerCaller)
	if err != nil {
		return nil, err
	}
	if publisher && surface == execution.Administration {
		invocation.administration = AdministrationContext{session: session}
	}
	return invocation, nil
}
