package connection

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/publication"
	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
)

// BindPublisherRecipient checks a fresh private recipient against this SAME
// original stream. It grants neither JOIN admission nor authenticated transfer;
// original-key Continuity, finite proposals and final checks remain mandatory.
func (stream *Stream) BindPublisherRecipient(ctx context.Context, operation *executionruntime.Operation, proof publication.Proof, profile [32]byte, request capsule.Request, digest [32]byte, end time.Time) (*RecipientBinding, error) {
	if stream == nil || stream.self != stream || ctx == nil || operation == nil {
		return nil, errIntegrity
	}
	var binding *RecipientBinding
	err := stream.commit(func() error {
		if stream.owner.operation != operation || stream.owner.role != wire.RolePublisher || stream.publisherRoute == nil || stream.send.generation == ^uint64(0) ||
			request.Revision < stream.recipientRevision || !time.Now().Before(time.Unix(stream.owner.identity.NoNewRecoveryAfter, 0)) {
			return errIntegrity
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		var err error
		binding, err = deriveRecipientGeneration(proof, profile, request, digest, end, time.Now(), stream.send.generation+1)
		if err != nil {
			return err
		}
		if binding.identity != stream.owner.identity || binding.logical != stream.owner.logical {
			return errIntegrity
		}
		binding.ctx, binding.operation = ctx, operation
		return binding.Check(ctx, operation)
	})
	if err != nil {
		return nil, err
	}
	return binding, nil
}

// PublisherContext is a copied full-tuple commitment for private dispatch,
// never a permission, accepted generation or successful recovery flag.
func (stream *Stream) PublisherContext() ([32]byte, error) {
	if stream == nil || stream.self != stream {
		return [32]byte{}, errIntegrity
	}
	var logical [32]byte
	err := stream.commit(func() error {
		if stream.owner.role != wire.RolePublisher || stream.publisherRoute == nil {
			return errIntegrity
		}
		logical = stream.owner.logical
		return nil
	})
	return logical, err
}

// Joined observes original physical completion, distinct from Application Done.
func (stream *Stream) Joined() <-chan struct{} {
	if stream == nil || stream.self != stream {
		return nil
	}
	return stream.joined
}

// JoinedCleanup returns the retained physical cleanup result only after this
// original stream has joined. A logical refusal is not a cleanup failure.
func (stream *Stream) JoinedCleanup() error {
	if stream == nil || stream.self != stream {
		return errIntegrity
	}
	select {
	case <-stream.joined:
		return stream.cleanupErr
	default:
		return errors.New("service Connection physical cleanup has not joined")
	}
}

// Stop synchronously seals local effects without returning physical borrows.
// Close still joins the original actors, callbacks and retained resources.
func (stream *Stream) Stop() {
	if stream == nil || stream.self != stream {
		return
	}
	stream.mu.Lock()
	stream.closing = true
	if stream.recovery != nil {
		stream.recovery.stop()
	}
	stream.cancel()
	stream.signalLocked()
	stream.mu.Unlock()
}
