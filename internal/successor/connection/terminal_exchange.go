package connection

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
)

var errIntegrity = errors.New("service Connection integrity contradiction")

// terminalExchange is the two-direction control ledger. Its caller holds the
// logical stream lock and supplies checked current-generation offsets. It
// admits no transport, permission, peer identity or replacement by itself.
type terminalExchange struct {
	generation                               uint64
	localClosed, remoteClosed                bool
	remoteTerminalCurrent                    bool
	localOffset, remoteOffset                uint64
	terminalWriting, terminalWritten         bool
	receiptWriting, receiptWritten           bool
	receiptReceived, receiptConfirmed        bool
	confirmationWriting, confirmationWritten bool
	writeSerial                              uint64
	active                                   *terminalWrite
}

// terminalWrite is a private single-use ticket pinned to the original
// generation and exact control fields before physical I/O begins.
type terminalWrite struct {
	self               *terminalWrite
	owner              *terminalExchange
	generation, serial uint64
	record             wire.StreamRecord
	consumed           bool
}

func newTerminalExchange(generation uint64) (*terminalExchange, error) {
	if generation == 0 {
		return nil, errIntegrity
	}
	return &terminalExchange{generation: generation}, nil
}

func (x *terminalExchange) closeLocal(offset uint64) error {
	if x.localClosed && x.localOffset != offset {
		return errIntegrity
	}
	x.localClosed, x.localOffset = true, offset
	return nil
}

func (x *terminalExchange) receiveTerminal(value wire.Terminal, receiveNext uint64, pending bool) (firstEOF bool, err error) {
	if value.AttachmentGeneration != x.generation || value.Offset != receiveNext || pending ||
		x.remoteClosed && x.remoteOffset != value.Offset {
		return false, errIntegrity
	}
	firstEOF = !x.remoteClosed
	x.remoteClosed, x.remoteOffset = true, value.Offset
	x.remoteTerminalCurrent = true
	return firstEOF, nil
}

func (x *terminalExchange) receiveAcknowledgement(value wire.Acknowledgement) error {
	if value.AttachmentGeneration != x.generation || !value.Terminal {
		return errIntegrity
	}
	if value.TerminalConfirmation {
		if !x.remoteClosed || value.Offset != x.remoteOffset || !x.receiptWritten && !x.receiptWriting {
			return errIntegrity
		}
		x.receiptConfirmed = true
		return nil
	}
	if !x.localClosed || value.Offset != x.localOffset || !x.terminalWritten && !x.terminalWriting {
		return errIntegrity
	}
	x.receiptReceived = true
	return nil
}

// beginWrite chooses the next causally permitted control after the caller has
// serialized all accepted Data. There is at most one active physical writer.
// Receipt and confirmation may arrive while its matching write is in progress.
func (x *terminalExchange) beginWrite(sentNext uint64) (*terminalWrite, error) {
	if x.active != nil {
		return nil, errors.New("terminal control writer already retained")
	}
	var record wire.StreamRecord
	switch {
	case x.localClosed && !x.terminalWritten:
		if sentNext != x.localOffset {
			return nil, nil
		}
		record.Terminal = &wire.Terminal{AttachmentGeneration: x.generation, Offset: x.localOffset}
	case x.remoteTerminalCurrent && !x.receiptWritten:
		record.Acknowledgement = &wire.Acknowledgement{AttachmentGeneration: x.generation, Offset: x.remoteOffset, Terminal: true, TerminalConfirmation: false}
	case x.receiptReceived && !x.confirmationWritten:
		record.Acknowledgement = &wire.Acknowledgement{AttachmentGeneration: x.generation, Offset: x.localOffset, Terminal: true, TerminalConfirmation: true}
	default:
		return nil, nil
	}
	if x.writeSerial == ^uint64(0) {
		return nil, errIntegrity
	}
	x.writeSerial++
	ticket := &terminalWrite{owner: x, generation: x.generation, serial: x.writeSerial, record: record}
	ticket.self = ticket
	x.active = ticket
	if record.Terminal != nil {
		x.terminalWriting = true
	} else if record.Acknowledgement.TerminalConfirmation {
		x.confirmationWriting = true
	} else {
		x.receiptWriting = true
	}
	return ticket, nil
}

// finishWrite refuses reused/foreign tickets and ignores a stale completion
// after replacement. A physical failure remains the stream owner's failure;
// it cannot become a successfully written control fact.
func (x *terminalExchange) finishWrite(ticket *terminalWrite, physicalErr error) (current bool, err error) {
	if ticket == nil || ticket.self != ticket || ticket.owner != x || ticket.consumed {
		return false, errIntegrity
	}
	if ticket.generation != x.generation {
		ticket.consumed = true
		return false, physicalErr
	}
	if x.active != ticket || ticket.serial != x.writeSerial {
		return false, errIntegrity
	}
	ticket.consumed = true
	x.active = nil
	if ticket.record.Terminal != nil {
		x.terminalWriting = false
		if physicalErr == nil {
			x.terminalWritten = true
		}
	} else if ticket.record.Acknowledgement.TerminalConfirmation {
		x.confirmationWriting = false
		if physicalErr == nil {
			x.confirmationWritten = true
		}
	} else {
		x.receiptWriting = false
		if physicalErr == nil {
			x.receiptWritten = true
		}
	}
	return true, physicalErr
}

// replace is called only after authenticated Continuity and live authority
// commit at the surrounding owner. It renews control proofs, never Application
// EOF or accepted byte counters. Every generation advances by exactly one.
func (x *terminalExchange) replace(generation uint64) error {
	if x.generation == ^uint64(0) || generation != x.generation+1 {
		return errIntegrity
	}
	x.generation = generation
	x.active = nil
	x.remoteTerminalCurrent = false
	x.terminalWriting, x.terminalWritten = false, false
	x.receiptWriting, x.receiptWritten = false, false
	x.receiptReceived, x.receiptConfirmed = false, false
	x.confirmationWriting, x.confirmationWritten = false, false
	return nil
}

func (x *terminalExchange) complete(sendAcknowledged uint64) bool {
	return x.localClosed && x.remoteClosed && x.remoteTerminalCurrent && sendAcknowledged == x.localOffset &&
		x.terminalWritten && x.receiptWritten && x.receiptReceived && x.receiptConfirmed &&
		x.confirmationWritten && x.active == nil
}
