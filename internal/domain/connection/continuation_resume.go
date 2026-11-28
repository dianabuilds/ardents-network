package connection

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

const (
	initialAttachmentGeneration uint64 = 1
	segmentOpeningTimeout              = 30 * time.Second
)

// segmentOpening authenticates the first or subsequent segment before
// either record actor starts.
type segmentOpening struct {
	ctx        context.Context
	reader     *Stream
	start      bool
	allowStart func() error
	resolve    func([32]byte) (*Stream, error)
	selected   *Stream
}

func (opening *segmentOpening) confirm(owner *AuthenticatedAttachment) (*segmentBoundaryExchange, error) {
	ctx := opening.ctx
	if ctx == nil {
		ctx = owner.context()
	}
	bounded, cancel := context.WithTimeout(ctx, segmentOpeningTimeout)
	defer cancel()
	if err := bounded.Err(); err != nil {
		return nil, err
	}
	joined := make(chan struct{})
	stop := context.AfterFunc(bounded, func() {
		defer close(joined)
		_ = owner.Close()
	})
	segment, err := opening.exchange(owner, bounded)
	if !stop() {
		<-joined
	}
	if err := errors.Join(err, bounded.Err()); err != nil {
		return nil, err
	}
	return segment, nil
}

func (opening *segmentOpening) exchange(owner *AuthenticatedAttachment, ctx context.Context) (*segmentBoundaryExchange, error) {
	if err := owner.check(); err != nil {
		return nil, err
	}
	physical := attachmentIO{owner: owner, initiator: owner.initiator, service: owner.service, context: owner.attachment}
	switch owner.role {
	case wire.RoleInitiator:
		if opening.resolve != nil || opening.start == (opening.reader != nil) {
			return nil, errContinuationRefused
		}
		base := wire.SegmentBoundary{Next: 1}
		if opening.start {
			request := wire.Continuation{Kind: wire.StartTransfer, AttachmentGeneration: initialAttachmentGeneration, LogicalContext: owner.logical}
			if err := exchangeTransferOpening(physical, request, wire.AcceptTransfer); err != nil {
				return nil, err
			}
		} else {
			request, err := opening.reader.readerResumeRequest(owner)
			if err != nil {
				return nil, err
			}
			if err := exchangeReaderResume(physical, request); err != nil {
				return nil, err
			}
			base = request.Boundary
		}
		return newSegmentBoundaryExchange(owner.role, base, [32]byte{})
	case wire.RolePublisher:
		if opening.reader != nil {
			return nil, errContinuationRefused
		}
		return opening.confirmPublisher(ctx, owner, physical)
	default:
		return nil, errContinuationRefused
	}
}

func (opening *segmentOpening) confirmPublisher(ctx context.Context, owner *AuthenticatedAttachment, physical attachmentIO) (*segmentBoundaryExchange, error) {
	var ticket [32]byte
	if _, err := rand.Read(ticket[:]); err != nil {
		return nil, err
	}
	request, err := readTransferOpening(physical)
	if err != nil || request.AttachmentGeneration != initialAttachmentGeneration || request.LogicalContext != owner.logical {
		return nil, errors.Join(errContinuationRefused, err)
	}
	base := wire.SegmentBoundary{Next: 1}
	answer := request
	switch request.Kind {
	case wire.StartTransfer:
		if !opening.start {
			return nil, errContinuationRefused
		}
		if opening.allowStart != nil {
			if err := opening.allowStart(); err != nil {
				return nil, err
			}
		}
		answer.Kind = wire.AcceptTransfer
	case wire.ResumeTransfer:
		if opening.resolve == nil {
			return nil, errContinuationRefused
		}
		previous, err := opening.resolve(request.Ticket)
		if err != nil || previous == nil {
			return nil, errors.Join(errContinuationRefused, err)
		}
		// Reader starts the fresh admission only after its own joined tail.
		// Wait briefly under this exact Attachment lifetime for the Publisher
		// side of the already authenticated original boundary to join.
		select {
		case <-previous.watcherDone:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		claim, err := previous.publisherContinuation(owner.operation.Context())
		if err != nil {
			return nil, err
		}
		answer, err = claim.bind(owner, initialAttachmentGeneration, request)
		if err != nil {
			return nil, err
		}
		base, opening.selected = answer.Boundary, previous
	default:
		return nil, errContinuationRefused
	}
	if err := wire.Write(physical, wire.Record{Continuation: &answer}); err != nil {
		return nil, err
	}
	return newSegmentBoundaryExchange(owner.role, base, ticket)
}

func (previous *Stream) readerResumeRequest(next *AuthenticatedAttachment) (wire.Continuation, error) {
	if previous == nil || previous.self != previous || previous.owner == nil ||
		previous.owner.role != wire.RoleInitiator || next == nil || next.role != wire.RoleInitiator ||
		next.operation != previous.owner.operation ||
		next.logical == [32]byte{} || next.logical == previous.owner.logical ||
		!sameContinuationService(previous.owner.identity, next.identity) {
		return wire.Continuation{}, errContinuationRefused
	}
	select {
	case <-previous.watcherDone:
	default:
		return wire.Continuation{}, errContinuationRefused
	}
	if previous.JoinedCleanup() != nil || previous.closeErr != nil || previous.segmentBoundary == nil {
		return wire.Continuation{}, errContinuationRefused
	}
	return wire.Continuation{
		Kind: wire.ResumeTransfer, AttachmentGeneration: initialAttachmentGeneration,
		Boundary: previous.segmentBoundary.boundary, Ticket: previous.segmentBoundary.ticket,
		LogicalContext: next.logical,
	}, nil
}

func exchangeReaderResume(physical io.ReadWriter, request wire.Continuation) error {
	if request.Kind != wire.ResumeTransfer {
		return errContinuationRefused
	}
	return exchangeTransferOpening(physical, request, wire.AcceptResume)
}

func exchangeTransferOpening(physical io.ReadWriter, request wire.Continuation, expected wire.ContinuationKind) error {
	if err := wire.Write(physical, wire.Record{Continuation: &request}); err != nil {
		return err
	}
	answer, err := readResumeRecord(physical, expected)
	if err != nil {
		return err
	}
	request.Kind = expected
	if answer != request {
		return errContinuationRefused
	}
	return nil
}

func readTransferOpening(physical io.Reader) (wire.Continuation, error) {
	record, err := wire.Read(physical)
	if err != nil || record.Continuation == nil {
		return wire.Continuation{}, errors.Join(errContinuationRefused, err)
	}
	return *record.Continuation, nil
}

func readResumeRecord(physical io.Reader, kind wire.ContinuationKind) (wire.Continuation, error) {
	record, err := readTransferOpening(physical)
	if err != nil || record.Kind != kind {
		return wire.Continuation{}, errors.Join(errContinuationRefused, err)
	}
	return record, nil
}
