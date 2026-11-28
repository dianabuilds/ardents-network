package connection

import (
	"math"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/transfer"
	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

// segmentBoundaryExchange is serialized by Stream.mu. Replay keeps the
// agreed boundary but requires current-generation physical evidence again.
type segmentBoundaryExchange struct {
	role wire.Role
	base wire.SegmentBoundary

	draining   bool
	localEnd   uint64
	peerEnd    uint64
	peerEndSet bool
	generation uint64
	sent       map[wire.ContinuationKind]bool
	received   map[wire.ContinuationKind]bool
	active     *segmentControlWrite

	proposal wire.SegmentBoundary
	ticket   [32]byte
}

type segmentControlWrite struct {
	owner  *segmentBoundaryExchange
	record wire.Continuation
}

func newSegmentBoundaryExchange(role wire.Role, base wire.SegmentBoundary, ticket [32]byte) (*segmentBoundaryExchange, error) {
	if base.Next == 0 || base.Next == math.MaxUint64 || base.EOF&^byte(3) != 0 ||
		role != wire.RoleInitiator && role != wire.RolePublisher ||
		(role == wire.RolePublisher) != (ticket != [32]byte{}) {
		return nil, errIntegrity
	}
	return &segmentBoundaryExchange{
		role: role, base: base, ticket: ticket,
		sent: make(map[wire.ContinuationKind]bool), received: make(map[wire.ContinuationKind]bool),
	}, nil
}

func (exchange *segmentBoundaryExchange) freeze(flow *transfer.State) {
	exchange.draining = true
	exchange.localEnd = flow.FreezeSegment()
}

func (exchange *segmentBoundaryExchange) observeGeneration(generation uint64) error {
	if generation == 0 || generation < exchange.generation {
		return errIntegrity
	}
	if generation > exchange.generation {
		exchange.generation = generation
		exchange.active = nil
		clear(exchange.sent)
		clear(exchange.received)
	}
	return nil
}

func (exchange *segmentBoundaryExchange) receive(flow *transfer.State, record wire.Continuation) error {
	if record.AttachmentGeneration != flow.Generation() {
		return errIntegrity
	}
	if err := exchange.observeGeneration(flow.Generation()); err != nil {
		return err
	}
	if record.Kind == wire.DrainSegment {
		if exchange.peerEndSet && record.DrainOffset != exchange.peerEnd ||
			record.DrainOffset < flow.ReplayOffsets().ReceiveNext {
			return errIntegrity
		}
		exchange.peerEnd, exchange.peerEndSet = record.DrainOffset, true
		exchange.freeze(flow)
		exchange.received[record.Kind] = true
		return nil
	}
	if !exchange.received[wire.DrainSegment] || record.Boundary.Next != exchange.base.Next+1 {
		return errIntegrity
	}
	switch record.Kind {
	case wire.ProposeBoundary:
		if exchange.role != wire.RolePublisher || exchange.proposal.Valid() && exchange.proposal != record.Boundary {
			return errIntegrity
		}
		exchange.proposal = record.Boundary
	case wire.OfferContinuation:
		if exchange.role != wire.RoleInitiator || !exchange.offered(wire.ProposeBoundary) ||
			exchange.proposal != record.Boundary || exchange.ticket != [32]byte{} && exchange.ticket != record.Ticket {
			return errIntegrity
		}
		exchange.ticket = record.Ticket
	case wire.ConfirmBoundary:
		if exchange.role != wire.RolePublisher || !exchange.offered(wire.OfferContinuation) ||
			!exchange.matches(record) {
			return errIntegrity
		}
	case wire.AcceptBoundary:
		if exchange.role != wire.RoleInitiator || !exchange.offered(wire.ConfirmBoundary) ||
			!exchange.matches(record) {
			return errIntegrity
		}
	default:
		return errIntegrity
	}
	exchange.received[record.Kind] = true
	return nil
}

func (exchange *segmentBoundaryExchange) matches(record wire.Continuation) bool {
	return record.Boundary == exchange.proposal && record.Ticket == exchange.ticket
}

// A peer can observe a record while the original writer is still returning.
// Retiring the boundary separately requires that write to have succeeded.
func (exchange *segmentBoundaryExchange) offered(kind wire.ContinuationKind) bool {
	return exchange.sent[kind] || exchange.active != nil && exchange.active.record.Kind == kind
}

func (exchange *segmentBoundaryExchange) beginWrite(flow *transfer.State) (*segmentControlWrite, error) {
	if err := exchange.observeGeneration(flow.Generation()); err != nil {
		return nil, err
	}
	if exchange.active != nil {
		return nil, errIntegrity
	}
	if !exchange.draining {
		return nil, nil
	}
	record := wire.Continuation{AttachmentGeneration: flow.Generation()}
	if !exchange.sent[wire.DrainSegment] {
		record.Kind, record.DrainOffset = wire.DrainSegment, exchange.localEnd
	} else if exchange.received[wire.DrainSegment] {
		if err := exchange.nextBoundaryRecord(flow, &record); err != nil {
			return nil, err
		}
	}
	if record.Kind == 0 {
		return nil, nil
	}
	ticket := &segmentControlWrite{owner: exchange, record: record}
	exchange.active = ticket
	return ticket, nil
}

func (exchange *segmentBoundaryExchange) nextBoundaryRecord(flow *transfer.State, record *wire.Continuation) error {
	positions, ok := flow.DrainedSegment()
	if !ok {
		return nil
	}
	if positions.Received != exchange.peerEnd {
		// DRAIN can arrive before the local Application finishes consuming
		// the peer's last queued bytes.
		return nil
	}
	boundary, err := exchange.absoluteBoundary(positions)
	if err != nil {
		return err
	}
	if exchange.proposal.Valid() && exchange.proposal != boundary {
		return errIntegrity
	}
	record.Boundary = boundary
	if exchange.role == wire.RoleInitiator {
		if !exchange.sent[wire.ProposeBoundary] {
			exchange.proposal = boundary
			record.Kind = wire.ProposeBoundary
		} else if exchange.received[wire.OfferContinuation] && !exchange.sent[wire.ConfirmBoundary] {
			record.Kind, record.Ticket = wire.ConfirmBoundary, exchange.ticket
		}
	} else if exchange.received[wire.ProposeBoundary] {
		if !exchange.sent[wire.OfferContinuation] {
			record.Kind, record.Ticket = wire.OfferContinuation, exchange.ticket
		} else if exchange.received[wire.ConfirmBoundary] && !exchange.sent[wire.AcceptBoundary] {
			record.Kind, record.Ticket = wire.AcceptBoundary, exchange.ticket
		}
	}
	return nil
}

func (exchange *segmentBoundaryExchange) finishWrite(ticket *segmentControlWrite, generation uint64, physicalErr error) error {
	if ticket == nil || ticket != exchange.active || ticket.owner != exchange ||
		ticket.record.AttachmentGeneration != generation || generation != exchange.generation {
		return errIntegrity
	}
	exchange.active = nil
	if physicalErr == nil {
		exchange.sent[ticket.record.Kind] = true
	}
	return physicalErr
}

func (exchange *segmentBoundaryExchange) awaitingFinalWrite(generation uint64) bool {
	if exchange.generation != generation || exchange.active == nil {
		return false
	}
	switch exchange.active.record.Kind {
	case wire.ConfirmBoundary:
		return exchange.received[wire.AcceptBoundary]
	case wire.AcceptBoundary:
		return exchange.received[wire.ConfirmBoundary]
	default:
		return false
	}
}

func (exchange *segmentBoundaryExchange) complete(generation uint64) bool {
	if exchange.generation != generation || exchange.active != nil {
		return false
	}
	if exchange.role == wire.RoleInitiator {
		return exchange.sent[wire.ConfirmBoundary] && exchange.received[wire.AcceptBoundary]
	}
	return exchange.received[wire.ConfirmBoundary] && exchange.sent[wire.AcceptBoundary]
}

func (exchange *segmentBoundaryExchange) absoluteBoundary(positions transfer.SegmentPositions) (wire.SegmentBoundary, error) {
	toPublisher, toReader := positions.Sent, positions.Received
	publisherEOF, readerEOF := positions.SentEOF, positions.ReceivedEOF
	if exchange.role == wire.RolePublisher {
		toPublisher, toReader = toReader, toPublisher
		publisherEOF, readerEOF = readerEOF, publisherEOF
	}
	base := exchange.base
	if toPublisher > math.MaxUint64-base.ToPublisher || toReader > math.MaxUint64-base.ToReader ||
		base.EOF&1 != 0 && toPublisher != 0 || base.EOF&2 != 0 && toReader != 0 {
		return wire.SegmentBoundary{}, errIntegrity
	}
	boundary := wire.SegmentBoundary{
		Next: base.Next + 1, ToPublisher: base.ToPublisher + toPublisher,
		ToReader: base.ToReader + toReader, EOF: base.EOF,
	}
	if publisherEOF {
		boundary.EOF |= 1
	}
	if readerEOF {
		boundary.EOF |= 2
	}
	return boundary, nil
}

// After observing the peer's committed cut, replay may include already
// admitted bytes but cannot extend that cut's Application range.
func (exchange *segmentBoundaryExchange) acceptsData(record wire.Data) bool {
	if exchange.peerEndSet && (record.Offset > exchange.peerEnd ||
		uint64(len(record.Payload)) > exchange.peerEnd-record.Offset) {
		return false
	}
	if !exchange.proposal.Valid() {
		return true
	}
	base, limit := exchange.base.ToReader, exchange.proposal.ToReader
	if exchange.role == wire.RolePublisher {
		base, limit = exchange.base.ToPublisher, exchange.proposal.ToPublisher
	}
	return limit >= base && record.Offset <= limit-base && uint64(len(record.Payload)) <= limit-base-record.Offset
}

// Terminal is an Application effect. A peer may repeat an authenticated EOF
// during replay, but cannot introduce one after positions have been agreed.
func (exchange *segmentBoundaryExchange) acceptsTerminal(record wire.Terminal) bool {
	if exchange.peerEndSet && record.Offset != exchange.peerEnd {
		return false
	}
	if !exchange.proposal.Valid() {
		return true
	}
	eof := byte(2)
	if exchange.role == wire.RolePublisher {
		eof = 1
	}
	return exchange.proposal.EOF&eof != 0
}
