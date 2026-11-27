package connection

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
	"github.com/dianabuilds/ardents-network/internal/domain/connection/transfer"
	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
)

// Stream owns the logical byte and Terminal state of one authenticated
// Connection. Physical authentication remains with its exact retained owner;
// neither a supplied transport nor copied offsets can construct this stream.
// Recovery source composition and replacement are additional admission work,
// not permission to expose an incompletely composed ordinary Service path.
type Stream struct {
	self              *Stream
	owner             *AuthenticatedAttachment
	contract          executionruntime.StreamContract
	ctx               context.Context
	cancel            context.CancelFunc
	mu                sync.Mutex
	changed           chan struct{} // Stream.mu owns notification replacement.
	writeMu           sync.Mutex
	transfer          *transfer.State
	key               [32]byte
	failure           error
	closing           bool
	settled           bool
	outcome           chan endpointstream.Outcome
	records           *attachmentRecords
	publisherRoute    *publisherRecoveryRoute
	initiatorRoute    *InitiatorSource
	recipientRevision uint64
	recoveryUsers     sync.WaitGroup
	joined            chan struct{}
	settledDone       chan struct{}
	watcherDone       chan struct{}
	stop              sync.Once
	closeErr          error
	cleanupErr        error
	lastProgress      time.Time
	recovery          *transfer.RecoveryEpisode
	publisherProposal *PublisherReplacement
	initiatorProposal *initiatorReplacement
	prepareUsers      sync.WaitGroup
	recoveryCleanup   error
	recoveryLoss      error // First retired Attachment work failure, not logical permission.
}

// OpenStream consumes the exact completed initial authentication once and
// retains its original logical Instance/Execution borrow. Trusted composition
// must finish its Route-backed replacement source before Application handoff.
func (owner *AuthenticatedAttachment) OpenStream(contract executionruntime.StreamContract) (*Stream, error) {
	return owner.openStream(contract, nil, nil)
}

func (owner *AuthenticatedAttachment) openStream(contract executionruntime.StreamContract, publisherRoute *publisherRecoveryRoute, initiatorRoute *InitiatorSource) (*Stream, error) {
	if owner == nil || owner.self != owner {
		return nil, errIntegrity
	}
	sendLimit, receiveLimit, err := contract.Limits(owner.operation)
	if err != nil {
		return nil, err
	}
	flow, err := transfer.New(1, sendLimit, receiveLimit)
	if err != nil {
		return nil, err
	}
	// Full Instance/native and Route observations precede the original JOIN
	// handoff barrier and the consumer's state mutex.
	if err := errors.Join(owner.check(), publisherRoute.check(owner)); err != nil {
		return nil, err
	}
	var stream *Stream
	handoff := func(checkOriginal func() error) error {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		if owner.closed || owner.consumed || owner.key == [32]byte{} {
			return errIntegrity
		}
		if err := errors.Join(checkOriginal(), owner.checkInitialCommit()); err != nil {
			return err
		}
		// The original immutable workload contract is checked again at the
		// handoff, after all potentially blocking owner observations.
		currentSend, currentReceive, err := contract.Limits(owner.operation)
		if err != nil || currentSend != sendLimit || currentReceive != receiveLimit {
			return errors.Join(errIntegrity, err)
		}
		ctx, cancel := context.WithCancel(owner.context())
		stream = &Stream{owner: owner, contract: contract, ctx: ctx, cancel: cancel, transfer: flow, publisherRoute: publisherRoute,
			initiatorRoute: initiatorRoute, key: owner.key, recipientRevision: owner.recipientRevision, lastProgress: time.Now(), outcome: make(chan endpointstream.Outcome, 1), joined: make(chan struct{}), settledDone: make(chan struct{}), watcherDone: make(chan struct{})}
		stream.self = stream
		if initiatorRoute != nil {
			initiatorRoute.mu.Lock()
			if err := initiatorRoute.localLocked(); err != nil || initiatorRoute.owner != owner || initiatorRoute.stream != nil || owner.role != wire.RoleInitiator || publisherRoute != nil {
				initiatorRoute.mu.Unlock()
				cancel()
				return errors.Join(errIntegrity, err)
			}
			initiatorRoute.stream = stream
			initiatorRoute.mu.Unlock()
		}
		owner.consumed = true
		clear(owner.key[:])
		stream.records = &attachmentRecords{generation: 1,
			physical:   attachmentIO{owner: owner, initiator: owner.initiator, service: owner.service, context: owner.attachment},
			readerDone: make(chan struct{}), writerDone: make(chan struct{}), retired: make(chan struct{}), replaced: make(chan struct{})}
		go stream.readRecords(stream.records)
		go stream.writeRecords(stream.records)
		go stream.watchLifetime()
		return nil
	}
	if owner.role == wire.RolePublisher {
		err = owner.service.CommitStream(owner.context(), handoff)
	} else {
		err = owner.initiator.physical.CommitOriginal(owner.context(), handoff)
	}
	if err != nil {
		return nil, err
	}
	return stream, nil
}

func (stream *Stream) checkLocked() error {
	if stream.closing {
		return errors.Join(io.ErrClosedPipe, stream.failure)
	}
	if err := stream.ctx.Err(); err != nil {
		return errors.Join(stream.failure, context.Cause(stream.ctx))
	}
	if _, _, err := stream.contract.Limits(stream.owner.operation); err != nil {
		return err
	}
	return stream.owner.checkLogicalCommit()
}

// commit observes foreign authority before taking any state mutex. The exact
// retained Route owners then serialize this bounded local state transition;
// action must never observe another owner, wait, perform I/O or join.
func (stream *Stream) commit(action func() error) error {
	return stream.commitHandoff(action, nil)
}

// Delivery is bounded local memory work after the original final fences, under
// the same locks. It cannot observe another owner, perform I/O or wait.
func (stream *Stream) commitHandoff(action func() error, delivery func()) (result error) {
	if stream == nil || stream.self != stream || stream.owner == nil || action == nil {
		return errIntegrity
	}
	// Original-owner refusal can happen before the local handoff takes its
	// lock. Cancellation of a failed stream must not hide its retained cause.
	defer func() {
		if result != nil {
			stream.mu.Lock()
			first := stream.failure
			stream.mu.Unlock()
			if first != nil {
				result = errors.Join(first, result)
			}
		}
	}()
	if err := errors.Join(stream.owner.checkLogical(), stream.publisherRoute.check(stream.owner)); err != nil {
		return err
	}
	handoff := func(checkOriginal func() error) error {
		stream.mu.Lock()
		defer stream.mu.Unlock()
		if stream.initiatorRoute != nil {
			source := stream.initiatorRoute
			if source.self != source || stream.owner.role != wire.RoleInitiator || stream.owner.initiator == nil || stream.owner.initiator.binding == nil || stream.owner.initiator.binding.source != source {
				return errIntegrity
			}
			source.mu.Lock()
			defer source.mu.Unlock()
			if source.owner != stream.owner || source.stream != stream {
				return errIntegrity
			}
			if err := source.localLocked(); err != nil {
				return err
			}
		}
		if err := errors.Join(checkOriginal(), stream.checkLocked()); err != nil {
			return err
		}
		if err := action(); err != nil {
			return err
		}
		if err := errors.Join(checkOriginal(), stream.checkLocked()); err != nil {
			return err
		}
		if delivery != nil {
			delivery()
		}
		// Clean outcome is an Application handoff too. It follows the final
		// original checks and any byte delivery, never an unfinished action.
		stream.settleLocked()
		return nil
	}
	if stream.publisherRoute != nil {
		return stream.publisherRoute.responder.Commit(handoff)
	}
	if stream.initiatorRoute != nil {
		return stream.initiatorRoute.scope.Commit(handoff)
	}
	// A bounded non-recovering consumer retains its original physical JOIN.
	if stream.owner.role == wire.RolePublisher {
		return stream.owner.service.CommitStream(stream.ctx, handoff)
	}
	return stream.owner.initiator.physical.CommitOriginal(stream.ctx, handoff)
}

func (stream *Stream) changeLocked() <-chan struct{} {
	if stream.changed == nil {
		stream.changed = make(chan struct{})
	}
	return stream.changed
}

func (stream *Stream) signalLocked() {
	if stream.changed != nil {
		close(stream.changed)
	}
	stream.changed = make(chan struct{})
}

func (stream *Stream) waitChange(changed <-chan struct{}) {
	select {
	case <-changed:
	case <-stream.ctx.Done():
	}
}
