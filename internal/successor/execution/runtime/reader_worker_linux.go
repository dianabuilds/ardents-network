//go:build linux

package runtime

import (
	"bytes"
	"context"
	"errors"
	"sync"

	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/successor/execution"
)

// LaunchReader initializes one fixed Reader with a sole Text protocol reader.
// It grants no destination, Service authentication or Route acquisition.
func (owner *Owner) LaunchReader(ctx context.Context, principal [32]byte) (*Invocation, error) {
	return owner.launchInvocation(ctx, principal, execution.Connection, nil, false, true)
}

func (invocation *Invocation) observeReaderAttachment() {
	input := &readerConnectionInput{incoming: make(chan application.Stream), result: make(chan readerCompletion, 1), finish: make(chan struct{})}
	invocation.readerInput = input
	borrow := &readerWorkerBorrow{invocation: invocation}
	go func() {
		defer close(invocation.observationDone)
		ready := make(chan []byte)
		joined := make(chan readerCompletion, 1)
		go func() {
			body, err := textdocument.AwaitRetainedWorkerConnection(invocation.readerCaller, borrow, input.incoming, ready, input.finish)
			joined <- readerCompletion{body: body, err: err}
		}()
		var completion readerCompletion
		select {
		case body := <-ready:
			invocation.useMu.Lock()
			input.completed = true
			invocation.useMu.Unlock()
			// Text still owns its protocol buffer through final IPC cleanup.
			// Keep the pending result independent, so failure erasure cannot
			// race with discarding the caller's sealed result.
			input.result <- readerCompletion{body: bytes.Clone(body)}
			completion = <-joined
		case completion = <-joined:
			input.result <- completion
		}
		err := completion.err
		clear(completion.body)
		invocation.observationError = err
		// The caller joins Source before releasing the successful RESULT
		// barrier. Text then joins its original IPC reader, followed by Job
		// retirement and the independently retained descendant cleanup.
		invocation.job.Retire()
		close(input.result)
	}()
}

// The initialized worker remains private. Its sole pending read may precede
// Operation creation; any returned protocol bytes and every write require the
// exact original Reader operation. Close captures final authority before I/O.
type readerWorkerBorrow struct {
	invocation *Invocation
	once       sync.Once
	closeErr   error
}

func (borrow *readerWorkerBorrow) checkOperation() error {
	invocation := borrow.invocation
	invocation.useMu.Lock()
	operation := invocation.operation
	invocation.useMu.Unlock()
	if operation == nil || operation.invocation != invocation {
		return errors.New("original Reader operation is unavailable")
	}
	return errors.Join(invocation.readerCaller.Err(), operation.CheckReader())
}

func (borrow *readerWorkerBorrow) Read(body []byte) (int, error) {
	if err := borrow.invocation.job.Check(); err != nil {
		return 0, err
	}
	n, err := borrow.invocation.activation.Attachment.Read(body)
	return n, errors.Join(err, borrow.checkOperation())
}

func (borrow *readerWorkerBorrow) Write(body []byte) (int, error) {
	if err := borrow.checkOperation(); err != nil {
		return 0, err
	}
	n, err := borrow.invocation.activation.Attachment.Write(body)
	return n, errors.Join(err, borrow.checkOperation())
}

func (borrow *readerWorkerBorrow) Close() error {
	borrow.once.Do(func() {
		authority := borrow.checkOperation()
		physical := borrow.invocation.activation.Attachment.Close()
		borrow.closeErr = errors.Join(authority, physical)
		borrow.invocation.useMu.Lock()
		operation := borrow.invocation.operation
		borrow.invocation.useMu.Unlock()
		if operation != nil {
			operation.RetainCleanup(physical)
		}
	})
	return borrow.closeErr
}
