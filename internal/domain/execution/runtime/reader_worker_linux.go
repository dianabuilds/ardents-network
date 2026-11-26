//go:build linux

package runtime

import (
	"bytes"
	"context"

	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/domain/execution"
)

// LaunchReader initializes one fixed Reader with a sole document protocol reader.
// It grants no destination, Service authentication or Route acquisition.
func (owner *Owner) LaunchReader(ctx context.Context, principal [32]byte) (*Invocation, error) {
	return owner.launchInvocation(ctx, principal, execution.Connection, nil, false, true)
}

func (invocation *Invocation) observeReaderAttachment() {
	input := &readerConnectionInput{incoming: make(chan application.Stream), result: make(chan readerCompletion, 1), finish: make(chan struct{})}
	invocation.readerInput = input
	// The initialized worker remains private. Its sole pending read may precede
	// Operation creation; any returned protocol bytes and every write require the
	// exact original Reader operation. The shared borrow keeps the Reader-specific
	// check hook (reader caller plus CheckReader); Close captures final authority
	// before I/O.
	borrow := &workerBorrow{
		invocation:  invocation,
		unavailable: "original Reader operation is unavailable",
		check:       readerOperationCheck,
	}
	go func() {
		defer close(invocation.observationDone)
		ready := make(chan []byte)
		joined := make(chan readerCompletion, 1)
		go func() {
			body, err := document.AwaitRetainedWorkerConnection(invocation.readerCaller, borrow, input.incoming, ready, input.finish)
			joined <- readerCompletion{body: body, err: err}
		}()
		var completion readerCompletion
		select {
		case body := <-ready:
			invocation.useMu.Lock()
			input.completed = true
			invocation.useMu.Unlock()
			// The document Application still owns its protocol buffer through final IPC cleanup.
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
		// barrier. The document Application then joins its original IPC reader, followed by Job
		// retirement and the independently retained descendant cleanup.
		invocation.job.Retire()
		close(input.result)
	}()
}
