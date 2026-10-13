package connection

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
)

const logicalQueueLimit = 256 << 10

var errBackpressure = errors.New("service Connection send replay queue is full")

// sendLedger owns one direction's accepted bytes across every Attachment.
// The surrounding stream serializes calls and checks original live authority
// before admission and after physical I/O. This ledger grants no authority.
type sendLedger struct {
	generation, limit           uint64
	base, end, next, exposedEnd uint64
	data                        []byte
	closed                      bool
	active                      *dataWrite
}

type dataWrite struct {
	self       *dataWrite
	owner      *sendLedger
	generation uint64
	record     wire.Data
	consumed   bool
}

func newSendLedger(generation, limit uint64) (*sendLedger, error) {
	if generation == 0 || limit == 0 {
		return nil, errIntegrity
	}
	return &sendLedger{generation: generation, limit: limit}, nil
}

func (ledger *sendLedger) accept(payload []byte) error {
	if err := ledger.checkAccept(payload); err != nil {
		return err
	}
	ledger.data = append(ledger.data, payload...)
	ledger.end += uint64(len(payload))
	return nil
}

// Admission planning changes neither accepted bytes nor the replay cursor.
// The Stream performs accept only after its original final handoff fences.
func (ledger *sendLedger) checkAccept(payload []byte) error {
	if ledger.closed || len(payload) == 0 || len(payload) > wire.MaximumDataBytes ||
		uint64(len(payload)) > ^uint64(0)-ledger.end || ledger.end+uint64(len(payload)) > ledger.limit {
		return errIntegrity
	}
	if len(payload) > logicalQueueLimit-len(ledger.data) {
		return errBackpressure
	}
	return nil
}

func (ledger *sendLedger) closeInput() uint64 {
	ledger.closed = true
	return ledger.end
}

// beginWrite replays retained bytes before admitting a later physical write.
// Copying the record prevents a concurrent ACK from changing its payload.
func (ledger *sendLedger) beginWrite() (*dataWrite, error) {
	if ledger.active != nil {
		return nil, errors.New("data writer already retained")
	}
	if ledger.next == ledger.end {
		return nil, nil
	}
	if ledger.next < ledger.base || ledger.next > ledger.end {
		return nil, errIntegrity
	}
	size := min(uint64(wire.MaximumDataBytes), ledger.end-ledger.next)
	start := ledger.next - ledger.base
	ticket := &dataWrite{owner: ledger, generation: ledger.generation, record: wire.Data{
		AttachmentGeneration: ledger.generation, Offset: ledger.next,
		Payload: append([]byte(nil), ledger.data[start:start+size]...),
	}}
	ticket.self = ticket
	ledger.active = ticket
	ledger.exposedEnd = max(ledger.exposedEnd, ledger.next+size)
	return ticket, nil
}

func (ledger *sendLedger) finishWrite(ticket *dataWrite, physicalErr error) (current bool, err error) {
	if ticket == nil || ticket.self != ticket || ticket.owner != ledger || ticket.consumed {
		return false, errIntegrity
	}
	if ticket.generation != ledger.generation {
		ticket.consumed = true
		return false, physicalErr
	}
	if ledger.active != ticket {
		return false, errIntegrity
	}
	ticket.consumed = true
	ledger.active = nil
	if physicalErr == nil {
		ledger.next = max(ledger.next, ticket.record.Offset+uint64(len(ticket.record.Payload)))
	}
	return true, physicalErr
}

// acknowledge requires the authenticated current generation and a range that
// has actually entered its physical writer, including an in-flight write.
func (ledger *sendLedger) acknowledge(generation, offset uint64) error {
	if generation != ledger.generation || offset > ledger.end || offset > ledger.exposedEnd {
		return errIntegrity
	}
	if offset <= ledger.base {
		return nil
	}
	trim := offset - ledger.base
	ledger.data = append(ledger.data[:0], ledger.data[trim:]...)
	ledger.base = offset
	ledger.next = max(ledger.next, offset)
	return nil
}

// replace is an authenticated Continuity commit, performed by the surrounding
// stream after joining the old writer. It never clears accepted byte counters,
// extends the byte limit, or repeats the Application request.
func (ledger *sendLedger) replace(generation, peerReceiveNext uint64) error {
	if err := ledger.checkReplacement(generation, peerReceiveNext); err != nil {
		return err
	}
	if err := ledger.acknowledge(ledger.generation, peerReceiveNext); err != nil {
		return err
	}
	ledger.generation = generation
	ledger.next = ledger.base
	// Continuity commits exactly the peer's retained ReceiveNext. An old
	// Attachment's writer exposure cannot authorize a larger ACK in this
	// generation before the suffix enters its own replay writer.
	ledger.exposedEnd = ledger.base
	ledger.active = nil
	return nil
}

func (ledger *sendLedger) checkReplacement(generation, peerReceiveNext uint64) error {
	if ledger.generation == ^uint64(0) || generation != ledger.generation+1 || peerReceiveNext < ledger.base ||
		peerReceiveNext > ledger.exposedEnd || peerReceiveNext > ledger.end {
		return errIntegrity
	}
	return nil
}
