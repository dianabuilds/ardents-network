//go:build linux

package runtime

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/domain/execution"
	"github.com/dianabuilds/ardents-network/internal/domain/execution/worker"
)

type byteStreamProtocol struct{ documentProtocol }

func (byteStreamProtocol) initialize(ctx context.Context, attachment *worker.Attachment, nonce [32]byte, _ string) error {
	return document.InitializeWorker(ctx, attachment, document.ByteStreamWorker, nonce, nil)
}

func (byteStreamProtocol) startObservation(invocation *Invocation) {
	invocation.byteStream = true
	input := &byteStreamInput{incoming: make(chan byteStreamExchange)}
	invocation.byteStreamInput = input
	go func() {
		defer close(invocation.observationDone)
		select {
		case <-invocation.job.Context().Done():
			return
		case exchange := <-input.incoming:
			ctx, cancel := context.WithCancel(invocation.job.Context())
			finished := make(chan struct{})
			stop := context.AfterFunc(exchange.ctx, func() { defer close(finished); cancel() })
			err := document.BridgeByteStreamWorker(ctx, invocation.activation.Attachment, exchange.local, exchange.remote)
			if !stop() {
				<-finished
			}
			cancel()
			invocation.observationError = err
			if err != nil {
				invocation.job.Retire()
			}
			exchange.done <- err
		}
	}()
}

// launchByteStream initializes the already verified worker through its fixed
// role entrypoint. The local authority still admits the original surface.
func (owner *Owner) launchByteStream(ctx context.Context, principal [32]byte, surface execution.Surface) (*Invocation, error) {
	switch surface {
	case execution.Connection, execution.Administration:
		return owner.launchApplicationInvocation(ctx, principal, surface, byteStreamProtocol{}, surface == execution.Administration, surface == execution.Connection)
	default:
		return nil, errors.New("byte-stream worker surface is invalid")
	}
}

// LaunchByteStreamReader admits the fixed Connection surface for a reader.
func (owner *Owner) LaunchByteStreamReader(ctx context.Context, principal [32]byte) (*Invocation, error) {
	return owner.launchByteStream(ctx, principal, execution.Connection)
}

// LaunchByteStreamPublisher admits the fixed Administration surface for publication.
func (owner *Owner) LaunchByteStreamPublisher(ctx context.Context, principal [32]byte) (*Invocation, error) {
	return owner.launchByteStream(ctx, principal, execution.Administration)
}
