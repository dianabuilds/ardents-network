package runtime

import (
	"context"
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/domain/connection"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
)

const maximumPublicationTransfers = 256

var errTransferUnavailable = errors.New("publisher transfer admission is unavailable")

// TransferIntake is the sole admission loop for a persistent Publication's
// Application transfers. Continuations go to their retained original stream;
// only a new authenticated transfer is returned to the worker's owner.
type TransferIntake struct {
	publisher *Publisher
	contract  executionruntime.StreamContract
	ctx       context.Context
	cancel    context.CancelFunc

	mu      sync.Mutex
	slots   map[[32]byte]*publisherTransferSlot
	closed  bool
	flights sync.WaitGroup
}

type publisherTransferSlot struct {
	ticket  [32]byte
	current *connection.Stream
	next    chan *connection.Stream
}

func (p *Publisher) OpenTransferIntake(ctx context.Context, contract executionruntime.StreamContract) (*TransferIntake, error) {
	if p == nil || p.self != p || ctx == nil || ctx.Err() != nil || p.check(ctx) != nil {
		return nil, errTransferUnavailable
	}
	if _, _, err := contract.Limits(p.config.Operation); err != nil {
		return nil, err
	}
	lifetime, cancel := context.WithCancel(ctx)
	return &TransferIntake{
		publisher: p, contract: contract, ctx: lifetime, cancel: cancel,
		slots: make(map[[32]byte]*publisherTransferSlot),
	}, nil
}

// Next serially authenticates new Attachments and dispatches any completed
// Resume before returning another new Application transfer.
func (intake *TransferIntake) Next() (*connection.Transfer, error) {
	publisher := intake.publisher
	publisher.mu.Lock()
	if publisher.closed {
		publisher.mu.Unlock()
		return nil, publisher.stoppedAcquisition(intake.ctx)
	}
	publisher.flights.Add(1)
	publisher.mu.Unlock()
	defer publisher.flights.Done()
	for {
		owner, err := intake.publisher.NextAuthentication(intake.ctx)
		if err != nil {
			return nil, err
		}
		stream, previous, err := owner.OpenPublisherTransferSegment(intake.ctx, intake.contract, intake.publisher.sourceScope, intake.publisher.responderScope, intake.checkCapacity, intake.resolve)
		if err != nil {
			cleanup := owner.Close()
			intake.publisher.config.Operation.RetainCleanup(cleanup)
			if cleanup != nil || intake.ctx.Err() != nil {
				return nil, errors.Join(err, cleanup, intake.ctx.Err())
			}
			continue
		}
		if err := intake.publisher.retainStream(intake.ctx, stream); err != nil {
			return nil, errors.Join(err, intake.release(stream))
		}
		ticket, err := stream.PublisherTransferTicket()
		if err != nil {
			return nil, errors.Join(err, intake.release(stream))
		}
		if previous != nil {
			if err := intake.forward(stream, previous, ticket); err != nil {
				cleanup := intake.release(stream)
				if cleanup != nil {
					return nil, errors.Join(err, cleanup)
				}
			}
			continue
		}
		transfer, err := intake.start(stream, ticket)
		if err != nil {
			return nil, errors.Join(err, intake.release(stream))
		}
		return transfer, nil
	}
}

func (intake *TransferIntake) checkCapacity() error {
	intake.mu.Lock()
	defer intake.mu.Unlock()
	if intake.closed || len(intake.slots) >= maximumPublicationTransfers {
		return errTransferUnavailable
	}
	return nil
}

func (intake *TransferIntake) resolve(ticket [32]byte) (*connection.Stream, error) {
	intake.mu.Lock()
	defer intake.mu.Unlock()
	slot := intake.slots[ticket]
	if intake.closed || slot == nil {
		return nil, errTransferUnavailable
	}
	return slot.current, nil
}

func (intake *TransferIntake) forward(stream, previous *connection.Stream, ticket [32]byte) error {
	previousTicket, err := previous.PublisherTransferTicket()
	if err != nil {
		return err
	}
	intake.mu.Lock()
	defer intake.mu.Unlock()
	slot := intake.slots[previousTicket]
	if intake.closed || slot == nil || slot.current != previous || intake.slots[ticket] != nil {
		return errTransferUnavailable
	}
	select {
	case slot.next <- stream:
		delete(intake.slots, previousTicket)
		slot.ticket, slot.current = ticket, stream
		intake.slots[ticket] = slot
		return nil
	default:
		return errTransferUnavailable
	}
}

func (intake *TransferIntake) start(stream *connection.Stream, ticket [32]byte) (*connection.Transfer, error) {
	intake.mu.Lock()
	defer intake.mu.Unlock()
	if intake.closed || len(intake.slots) >= maximumPublicationTransfers || intake.slots[ticket] != nil {
		return nil, errTransferUnavailable
	}
	slot := &publisherTransferSlot{ticket: ticket, current: stream, next: make(chan *connection.Stream, 1)}
	follow := func(ctx context.Context, _ *connection.Stream) (*connection.Stream, func() error, error) {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case next := <-slot.next:
			return next, func() error { return intake.release(next) }, nil
		}
	}
	owner, err := connection.OpenTransfer(intake.ctx, stream, func() error { return intake.release(stream) }, follow)
	if err != nil {
		return nil, err
	}
	intake.slots[ticket] = slot
	intake.flights.Add(1)
	go func() {
		defer intake.flights.Done()
		<-owner.Joined()
		intake.mu.Lock()
		delete(intake.slots, slot.ticket)
		intake.mu.Unlock()
		select {
		case orphan := <-slot.next:
			_ = intake.release(orphan)
		default:
		}
	}()
	return owner, nil
}

func (intake *TransferIntake) release(stream *connection.Stream) error {
	err := stream.Close()
	intake.publisher.config.Operation.RetainCleanup(stream.JoinedCleanup())
	return err
}

func (intake *TransferIntake) Close() {
	intake.cancel()
	intake.mu.Lock()
	intake.closed = true
	intake.mu.Unlock()
	intake.flights.Wait()
}
