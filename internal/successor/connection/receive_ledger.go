package connection

import (
	"bytes"
	"io"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
)

// receiveLedger retains one ordered direction through physical replacement.
// One bounded buffer holds both unread Application bytes and comparison history;
// consuming bytes moves the cursor without duplicating the retained payload.
// The stream owns synchronization, authenticated generation and live authority.
type receiveLedger struct {
	generation, limit  uint64
	base, next, readAt uint64
	data               []byte
	terminal           bool
}

func newReceiveLedger(generation, limit uint64) (*receiveLedger, error) {
	if generation == 0 || limit == 0 {
		return nil, errIntegrity
	}
	return &receiveLedger{generation: generation, limit: limit, data: make([]byte, 0, logicalQueueLimit)}, nil
}

// accept checks the complete duplicate prefix before admitting any fresh suffix.
// Reliable ordered Attachments and serialized replay cannot create forward gaps.
// A capacity refusal changes neither the byte counters nor comparison history;
// the stream must wait for Application consumption before retrying this record.
func (ledger *receiveLedger) accept(value wire.Data) error {
	if value.AttachmentGeneration != ledger.generation || len(value.Payload) == 0 || len(value.Payload) > wire.MaximumDataBytes ||
		uint64(len(value.Payload)) > ^uint64(0)-value.Offset {
		return errIntegrity
	}
	end := value.Offset + uint64(len(value.Payload))
	if end > ledger.limit || value.Offset > ledger.next || value.Offset < ledger.base {
		return errIntegrity
	}
	duplicateEnd := min(end, ledger.next)
	duplicateSize := duplicateEnd - value.Offset
	start := value.Offset - ledger.base
	if !bytes.Equal(ledger.data[start:start+duplicateSize], value.Payload[:duplicateSize]) {
		return errIntegrity
	}
	if end <= ledger.next {
		return nil
	}
	if ledger.terminal {
		return errIntegrity
	}
	fresh := value.Payload[duplicateSize:]
	if uint64(len(fresh)) > logicalQueueLimit-(ledger.next-ledger.readAt) {
		return errBackpressure
	}
	// Drop only already consumed history, keeping unread bytes and the newest
	// replay window in the same allocation. No Application effect is evicted.
	trim := max(0, len(ledger.data)+len(fresh)-logicalQueueLimit)
	if trim != 0 {
		copy(ledger.data, ledger.data[trim:])
		clear(ledger.data[len(ledger.data)-trim:])
		ledger.data = ledger.data[:len(ledger.data)-trim]
		ledger.base += uint64(trim)
	}
	ledger.data = append(ledger.data, fresh...)
	ledger.next = end
	return nil
}

// read presents each accepted byte once. EOF requires authenticated Terminal;
// an empty active queue supplies no synthetic Application EOF.
func (ledger *receiveLedger) read(destination []byte) (int, error) {
	n, err := ledger.peek(destination)
	ledger.readAt += uint64(n)
	return n, err
}

// peek stages bytes without consuming the Application cursor. Failed original
// handoff fences must leave those bytes available to their original ledger.
func (ledger *receiveLedger) peek(destination []byte) (int, error) {
	if len(destination) == 0 {
		return 0, nil
	}
	if ledger.readAt == ledger.next {
		if ledger.terminal {
			return 0, io.EOF
		}
		return 0, nil
	}
	start := ledger.readAt - ledger.base
	n := copy(destination, ledger.data[start:])
	return n, nil
}

func (ledger *receiveLedger) closeInput(value wire.Terminal) error {
	if value.AttachmentGeneration != ledger.generation || value.Offset != ledger.next {
		return errIntegrity
	}
	ledger.terminal = true
	return nil
}

// replace follows authenticated Continuity, after joining the old receiver.
// The sender must still retain every byte not yet received here. Nothing
// resets the accepted or Application cursors, Terminal or workload counter.
func (ledger *receiveLedger) replace(generation, peerSendBase, peerSendEnd uint64) error {
	if err := ledger.checkReplacement(generation, peerSendBase, peerSendEnd); err != nil {
		return err
	}
	ledger.generation = generation
	return nil
}

func (ledger *receiveLedger) checkReplacement(generation, peerSendBase, peerSendEnd uint64) error {
	if ledger.generation == ^uint64(0) || generation != ledger.generation+1 || peerSendBase > ledger.next || peerSendEnd < ledger.next || peerSendEnd > ledger.limit ||
		ledger.terminal && peerSendEnd != ledger.next {
		return errIntegrity
	}
	return nil
}
