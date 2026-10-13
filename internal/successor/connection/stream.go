package connection

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
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
	cond              *sync.Cond
	changed           chan struct{} // Stream.mu owns notification replacement.
	writeMu           sync.Mutex
	send              *sendLedger
	receive           *receiveLedger
	terminal          *terminalExchange
	key               [32]byte
	ackSerial         uint64
	ackWritten        uint64
	failure           error
	closing           bool
	settled           bool
	outcome           chan application.Outcome
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
	recovery          *recoveryEpisode
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
	send, err := newSendLedger(1, sendLimit)
	if err != nil {
		return nil, err
	}
	receive, err := newReceiveLedger(1, receiveLimit)
	if err != nil {
		return nil, err
	}
	terminal, err := newTerminalExchange(1)
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
		stream = &Stream{owner: owner, contract: contract, ctx: ctx, cancel: cancel, send: send, receive: receive, terminal: terminal, publisherRoute: publisherRoute,
			initiatorRoute: initiatorRoute, key: owner.key, recipientRevision: owner.recipientRevision, lastProgress: time.Now(), outcome: make(chan application.Outcome, 1), joined: make(chan struct{}), settledDone: make(chan struct{}), watcherDone: make(chan struct{})}
		stream.self = stream
		stream.cond = sync.NewCond(&stream.mu)
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
	if stream.cond != nil {
		stream.cond.Broadcast()
	}
}

func (stream *Stream) waitChange(changed <-chan struct{}) {
	select {
	case <-changed:
	case <-stream.ctx.Done():
	}
}

func (stream *Stream) Read(destination []byte) (int, error) {
	if stream == nil || stream.self != stream {
		return 0, errIntegrity
	}
	// Caller memory is an Application handoff. Keep bytes private until the
	// original owners' final local fences accept this exact read transition.
	staged := make([]byte, min(len(destination), wire.MaximumDataBytes))
	defer clear(staged)
	for {
		var n int
		var readErr error
		var changed <-chan struct{}
		done := false
		err := stream.commitHandoff(func() error {
			changed = stream.changeLocked()
			n, readErr = stream.receive.peek(staged)
			done = n != 0 || readErr != nil || len(destination) == 0
			return nil
		}, func() {
			n, readErr = stream.receive.read(staged)
			n, readErr = deliverRead(destination, staged, n, readErr, nil)
			if done {
				stream.signalLocked()
			}
		})
		if err != nil {
			return deliverRead(destination, staged, n, readErr, err)
		}
		if done {
			// Preserve the exact authenticated EOF sentinel for io.Reader.
			return n, readErr
		}
		stream.waitChange(changed)
	}
}

// deliverRead cannot grant authority; only Read supplies the actual commit
// result. A failed final fence exposes neither a positive count nor plaintext.
func deliverRead(destination, staged []byte, n int, readErr, handoffErr error) (int, error) {
	if handoffErr != nil {
		return 0, errors.Join(readErr, handoffErr)
	}
	if n < 0 || n > len(staged) || n > len(destination) {
		return 0, errIntegrity
	}
	copy(destination, staged[:n])
	return n, readErr
}

func (stream *Stream) Write(body []byte) (int, error) {
	if stream == nil || stream.self != stream {
		return 0, errIntegrity
	}
	stream.writeMu.Lock()
	defer stream.writeMu.Unlock()
	accepted := 0
	for {
		var changed <-chan struct{}
		var stagedSize int
		var deliveryErr error
		err := stream.commitHandoff(func() error {
			if stream.settled || stream.send.closed {
				return io.ErrClosedPipe
			}
			if accepted == len(body) {
				return nil
			}
			size := min(len(body)-accepted, wire.MaximumDataBytes)
			err := stream.send.checkAccept(body[accepted : accepted+size])
			if errors.Is(err, errBackpressure) {
				changed = stream.changeLocked()
				return nil
			}
			if err != nil {
				stream.failLocked(err)
				return err
			}
			stagedSize = size
			return nil
		}, func() {
			if stagedSize == 0 {
				return
			}
			deliveryErr = stream.send.accept(body[accepted : accepted+stagedSize])
			if deliveryErr != nil {
				stream.failLocked(deliveryErr)
				return
			}
			accepted += stagedSize
			stream.signalLocked()
		})
		err = errors.Join(err, deliveryErr)
		if err != nil || accepted == len(body) {
			return accepted, err
		}
		if changed != nil {
			stream.waitChange(changed)
		}
	}
}

func (stream *Stream) CloseInput() error {
	if stream == nil || stream.self != stream {
		return errIntegrity
	}
	stream.writeMu.Lock()
	defer stream.writeMu.Unlock()
	return stream.commit(func() error {
		if err := stream.terminal.closeLocal(stream.send.closeInput()); err != nil {
			stream.failLocked(err)
			return err
		}
		stream.signalLocked()
		return nil
	})
}

func (stream *Stream) Done() <-chan application.Outcome {
	if stream == nil || stream.self != stream {
		return nil
	}
	return stream.outcome
}

// Context retains the original logical cancellation and immutable safety bound.
// It exposes no cancel function or physical/protocol authority.
func (stream *Stream) Context() context.Context {
	if stream == nil || stream.self != stream {
		return nil
	}
	return stream.ctx
}

func (stream *Stream) failLocked(err error) {
	if err != nil && stream.failure == nil {
		stream.failure = err
	}
	stream.cancel()
	stream.signalLocked()
}

func (stream *Stream) settleLocked() {
	if stream.settled || stream.receive.readAt != stream.receive.next || !stream.terminal.complete(stream.send.base) {
		return
	}
	if err := stream.checkLocked(); err != nil {
		stream.failLocked(err)
		return
	}
	stream.settled = true
	stream.outcome <- application.Outcome{Class: application.CleanClose}
	close(stream.outcome)
	close(stream.settledDone)
	// Original authority/Instance remain retained during the control tail.
	// Explicit Close or the original lifetime interrupts and joins it.
	stream.signalLocked()
}

func (stream *Stream) watchLifetime() {
	defer close(stream.watcherDone)
	select {
	case <-stream.ctx.Done():
	case <-stream.settledDone:
		remaining := time.Until(time.Unix(stream.owner.identity.NoNewRecoveryAfter, 0))
		if remaining > 0 {
			timer := time.NewTimer(remaining)
			select {
			case <-stream.ctx.Done():
			case <-timer.C:
			}
			timer.Stop()
		}
		stream.sealRetirement()
	}
	stream.retire()
}

// sealRetirement denies late record effects before deliberate interruption.
// Cancellation alone leaves a window in which an interrupted record actor can
// replace an already settled outcome with context.Canceled. Physical failures
// remain with the exact joined Attachment/Route cleanup owners.
func (stream *Stream) sealRetirement() {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	stream.closing = true
	if stream.failure == nil && !stream.settled {
		stream.failure = context.Cause(stream.ctx)
		if stream.failure == nil {
			stream.failure = context.Canceled
		}
	}
	stream.cancel()
	stream.signalLocked()
}

func (stream *Stream) retire() {
	stream.stop.Do(func() {
		stream.mu.Lock()
		if stream.failure == nil && !stream.settled {
			stream.failure = context.Cause(stream.ctx)
		}
		stream.closing = true
		if stream.recovery != nil {
			stream.recovery.stop()
		}
		proposal := stream.publisherProposal
		initiatorProposal := stream.initiatorProposal
		stream.cancel()
		stream.signalLocked()
		stream.mu.Unlock()
		if proposal != nil {
			_ = proposal.Close()
		}
		if initiatorProposal != nil {
			_ = initiatorProposal.close()
		}
		_ = stream.owner.Close()
		physical := stream.owner.cleanupErr
		if stream.records.physical.initiator != nil && stream.records.physical.initiator != stream.owner.initiator {
			_ = stream.records.physical.initiator.Close()
			physical = errors.Join(physical, stream.records.physical.initiator.cleanupErr)
		}
		stream.prepareUsers.Wait()
		stream.recoveryUsers.Wait()
		// The original records retain their raw work results after this join.
		// Intentional interruption may return a socket error as well as caller
		// cancellation; it is not a second logical or physical cleanup failure.
		// Physical failures are retained by the exact lower cleanup owners.
		_ = stream.records.join()
		stream.mu.Lock()
		stream.cleanupErr = errors.Join(physical, stream.recoveryCleanup)
		stream.closeErr = errors.Join(stream.failure, stream.cleanupErr)
		clear(stream.key[:])
		if !stream.settled {
			stream.outcome <- application.Outcome{Class: application.IndeterminateFailure, Reason: "Service Connection did not complete authenticated Terminal exchange"}
			close(stream.outcome)
		}
		stream.mu.Unlock()
		close(stream.joined)
	})
}

func (stream *Stream) Close() error {
	if stream == nil {
		return nil
	}
	if stream.self != stream {
		return errIntegrity
	}
	stream.sealRetirement()
	stream.retire()
	<-stream.joined
	<-stream.watcherDone
	return stream.closeErr
}
