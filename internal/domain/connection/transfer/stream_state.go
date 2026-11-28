package transfer

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

var (
	// ErrIntegrity identifies an inconsistent logical Connection binding or transition.
	ErrIntegrity = errors.New("service Connection integrity contradiction")
	// ErrBackpressure means the current ordered transfer must wait for capacity.
	ErrBackpressure = errors.New("service Connection send replay queue is full")
)

// State retains both directions and the exact current exchange of Terminal
// records. The caller serializes all transitions under the logical Stream lock.
type State struct {
	send       *sendLedger
	receive    *receiveLedger
	terminal   *terminalExchange
	ackSerial  uint64
	ackWritten uint64
	draining   bool
}

func New(generation, sendLimit, receiveLimit uint64) (*State, error) {
	send, err := newSendLedger(generation, sendLimit)
	if err != nil {
		return nil, err
	}
	receive, err := newReceiveLedger(generation, receiveLimit)
	if err != nil {
		return nil, err
	}
	terminal, err := newTerminalExchange(generation)
	if err != nil {
		return nil, err
	}
	return &State{send: send, receive: receive, terminal: terminal}, nil
}

func (s *State) Generation() uint64 { return s.send.generation }

// ReplayOffsets reports the local bounded ranges used by authenticated Continuity.
type ReplayOffsets struct {
	SendBase, SendEnd, ReceiveNext uint64
}

func (s *State) ReplayOffsets() ReplayOffsets {
	return ReplayOffsets{SendBase: s.send.base, SendEnd: s.send.end, ReceiveNext: s.receive.next}
}

// Progress is comparable evidence of a change in accepted byte or control
// state, for the caller's existing recovery timer.
type Progress struct {
	sendBase, sentNext, receivedNext                                       uint64
	remoteClosed, remoteTerminalCurrent, receiptReceived, receiptConfirmed bool
	terminalWritten, receiptWritten, confirmationWritten                   bool
}

func (s *State) Progress() Progress {
	return Progress{s.send.base, s.send.next, s.receive.next, s.terminal.remoteClosed, s.terminal.remoteTerminalCurrent,
		s.terminal.receiptReceived, s.terminal.receiptConfirmed, s.terminal.terminalWritten, s.terminal.receiptWritten, s.terminal.confirmationWritten}
}

func (s *State) Peek(body []byte) (int, error) { return s.receive.peek(body) }
func (s *State) Read(body []byte) (int, error) { return s.receive.read(body) }
func (s *State) CheckWrite(body []byte) error {
	if s.draining {
		return ErrSegmentDraining
	}
	return s.send.checkAccept(body)
}
func (s *State) Write(body []byte) error {
	if s.draining {
		return ErrSegmentDraining
	}
	return s.send.accept(body)
}
func (s *State) InputClosed() bool { return s.send.closed }

func (s *State) CloseInput() error {
	if s.draining && !s.send.closed {
		return ErrSegmentDraining
	}
	return s.terminal.closeLocal(s.send.closeInput())
}

// AcceptRecord handles one already decoded record from the exact current
// Attachment. The caller owns original authority and rejects Data after
// logical settlement.
func (s *State) AcceptRecord(record wire.Record) error {
	switch {
	case record.Data != nil:
		if err := s.receive.accept(*record.Data); err != nil {
			return err
		}
		if s.ackSerial == ^uint64(0) {
			return ErrIntegrity
		}
		s.ackSerial++
	case record.Terminal != nil:
		if err := s.receive.closeInput(*record.Terminal); err != nil {
			return err
		}
		if _, err := s.terminal.receiveTerminal(*record.Terminal, s.receive.next, false); err != nil {
			return err
		}
	case record.Acknowledgement != nil:
		value := *record.Acknowledgement
		if value.Terminal {
			if err := s.terminal.receiveAcknowledgement(value); err != nil {
				return err
			}
			if value.TerminalConfirmation {
				return nil
			}
		}
		return s.send.acknowledge(value.AttachmentGeneration, value.Offset)
	default:
		return ErrIntegrity
	}
	return nil
}

// TerminalComplete reports the current authenticated control receipts. The
// Stream separately retains delivery, logical outcome and the physical tail.
func (s *State) TerminalComplete() bool { return s.terminal.complete(s.send.base) }

func (s *State) DeliveryComplete() bool {
	return s.receive.readAt == s.receive.next && s.TerminalComplete()
}

// RetireObservation finishes the counter contribution once on original retirement.
func (s *State) RetireObservation() {
	s.send.retireObservation()
	s.receive.retireObservation()
}
