package runtime

import (
	"context"
	"errors"

	application "github.com/dianabuilds/ardents-network/internal/application/connection"
)

// This unbuffered byte boundary belongs to the original Text protocol reader.
// It supplies no Service authority; Connection composition must authenticate
// and retain the original stream before attempting this local transfer.
type publisherConnectionInput struct {
	incoming chan application.Stream
}

// DeliverPublisherStream transfers one already authenticated stream to the
// exact original qualified snapshot invocation. Waiting remains sender-owned;
// received streams belong to the sole Text reader until their joined Close.
// transferred stays true if authority loss is observed after the actual send.
func (operation *Operation) DeliverPublisherStream(ctx context.Context, contract StreamContract, stream application.Stream) (transferred bool, result error) {
	if ctx == nil || stream == nil {
		return false, errors.New("publisher stream handoff is absent")
	}
	if err := errors.Join(ctx.Err(), operation.CheckPublisher()); err != nil {
		return false, err
	}
	if _, _, err := contract.Limits(operation); err != nil {
		return false, err
	}
	invocation := operation.invocation
	if invocation == nil {
		return false, errors.New("original Publisher invocation is absent")
	}
	invocation.useMu.Lock()
	input := invocation.publisherInput
	current := !invocation.closing && invocation.operation == operation && input != nil
	invocation.useMu.Unlock()
	if !current {
		return false, errors.New("original Publisher stream reader is unavailable")
	}
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-operation.Context().Done():
		return false, operation.Context().Err()
	case input.incoming <- stream:
		transferred = true
	}
	if _, _, err := contract.Limits(operation); err != nil {
		return true, errors.Join(err, stream.Close())
	}
	if err := errors.Join(ctx.Err(), operation.CheckPublisher()); err != nil {
		return true, errors.Join(err, stream.Close())
	}
	return true, nil
}
