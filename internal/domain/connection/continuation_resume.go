package connection

import (
	"crypto/rand"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

const initialAttachmentGeneration uint64 = 1

// segmentResume carries an original confirmed boundary into the initial
// authenticated Attachment flight. Exactly one role-specific owner is set.
type segmentResume struct {
	reader    *Stream
	publisher *continuationClaim
}

func (resume *segmentResume) confirm(owner *AuthenticatedAttachment) (*segmentBoundaryExchange, error) {
	if err := owner.check(); err != nil {
		return nil, err
	}
	physical := attachmentIO{owner: owner, initiator: owner.initiator, service: owner.service, context: owner.attachment}
	switch owner.role {
	case wire.RoleInitiator:
		if resume.reader == nil || resume.publisher != nil {
			return nil, errContinuationRefused
		}
		boundary, err := resume.reader.readerResumeRequest(owner)
		if err != nil {
			return nil, err
		}
		if err := exchangeReaderResume(physical, boundary); err != nil {
			return nil, err
		}
		return newSegmentBoundaryExchange(owner.role, boundary.Boundary, [32]byte{})
	case wire.RolePublisher:
		if resume.publisher == nil || resume.reader != nil {
			return nil, errContinuationRefused
		}
		var nextTicket [32]byte
		if _, err := rand.Read(nextTicket[:]); err != nil {
			return nil, err
		}
		request, err := readResumeRecord(physical, wire.ResumeTransfer)
		if err != nil {
			return nil, err
		}
		answer, err := resume.publisher.bind(owner, initialAttachmentGeneration, request)
		if err != nil {
			return nil, err
		}
		if err := wire.Write(physical, wire.Record{Continuation: &answer}); err != nil {
			return nil, err
		}
		return newSegmentBoundaryExchange(owner.role, answer.Boundary, nextTicket)
	default:
		return nil, errContinuationRefused
	}
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
	if err := wire.Write(physical, wire.Record{Continuation: &request}); err != nil {
		return err
	}
	answer, err := readResumeRecord(physical, wire.AcceptResume)
	if err != nil {
		return err
	}
	request.Kind = wire.AcceptResume
	if answer != request {
		return errContinuationRefused
	}
	return nil
}

func readResumeRecord(physical io.Reader, kind wire.ContinuationKind) (wire.Continuation, error) {
	record, err := wire.Read(physical)
	if err != nil || record.Continuation == nil || record.Continuation.Kind != kind {
		return wire.Continuation{}, errors.Join(errContinuationRefused, err)
	}
	return *record.Continuation, nil
}
