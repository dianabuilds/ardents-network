package runtime

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/execution"
)

type byteStreamInput struct {
	incoming chan byteStreamExchange
}

type byteStreamExchange struct {
	ctx           context.Context
	local, remote document.ByteStreamPort
	done          chan error
}

// ExchangeByteStream hands exactly one pair of ports to the original worker.
// A successful send transfers their use until the worker result joins. The
// caller still owns the Service's authenticated terminal result.
func (operation *Operation) ExchangeByteStream(ctx context.Context, contract StreamContract, local, remote document.ByteStreamPort) error {
	if ctx == nil || local == nil || remote == nil || operation == nil || !operation.UsesByteStream() {
		return errors.New("qualified byte-stream exchange is unavailable")
	}
	if _, _, err := contract.Limits(operation); err != nil {
		return err
	}
	input := operation.invocation.byteStreamInput
	if input == nil {
		return errors.New("original byte-stream worker is unavailable")
	}
	exchangeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(operation.Context(), cancel)
	defer stop()
	exchange := byteStreamExchange{ctx: exchangeCtx, local: local, remote: remote, done: make(chan error, 1)}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-operation.Context().Done():
		return operation.Context().Err()
	case <-operation.invocation.observationDone:
		return errors.New("original byte-stream worker was already used")
	case input.incoming <- exchange:
	}
	return <-exchange.done
}

// LaunchByteStreamReader admits the fixed Connection surface for a reader.
func (owner *Owner) LaunchByteStreamReader(ctx context.Context, principal [32]byte) (*Invocation, error) {
	return owner.LaunchByteStream(ctx, principal, execution.Connection)
}

// LaunchByteStreamPublisher admits the fixed Administration surface for publication.
func (owner *Owner) LaunchByteStreamPublisher(ctx context.Context, principal [32]byte) (*Invocation, error) {
	return owner.LaunchByteStream(ctx, principal, execution.Administration)
}
