package transfer

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

// CheckReplacement verifies the peer's retained ranges without changing active
// bytes. The independent original authority checks belong to the Connection.
func (s *State) CheckReplacement(generation uint64, peer wire.Continuity) error {
	return errors.Join(s.send.checkReplacement(generation, peer.ReceiveNext),
		s.receive.checkReplacement(generation, peer.SendBase, peer.SendEnd))
}

// CanReplace reports whether the current ledgers agree on their generation and
// have returned all Data and Terminal write tickets.
func (s *State) CanReplace() bool {
	return s.send.active == nil && s.terminal.active == nil && s.SameGeneration()
}

// SameGeneration reports the coherent identity used to plan Continuity.
func (s *State) SameGeneration() bool {
	return s.send.generation == s.receive.generation && s.send.generation == s.terminal.generation
}

// Replacement holds the proposed next generation. All calls happen during a
// single Stream mutex hold after the original physical record actors join.
type Replacement struct {
	owner    *State
	send     *sendLedger
	receive  *receiveLedger
	terminal *terminalExchange
}

// PrepareReplacement derives all three ledgers together without trimming the
// current replay allocation. The caller must finish the same recovery ticket
// before installing the result.
func (s *State) PrepareReplacement(generation uint64, peer wire.Continuity) (*Replacement, error) {
	send, sendErr := s.send.prepareReplacement(generation, peer.ReceiveNext)
	receive, terminal := *s.receive, *s.terminal
	if err := errors.Join(sendErr, receive.replace(generation, peer.SendBase, peer.SendEnd), terminal.replace(generation)); err != nil {
		if send != nil {
			clear(send.data)
		}
		return nil, err
	}
	return &Replacement{s, send, &receive, &terminal}, nil
}

// Commit installs the complete candidate under the unchanged original Stream lock.
func (p *Replacement) Commit() {
	if p.owner == nil {
		return
	}
	p.owner.send.finishReplacement(p.send)
	p.owner.send, p.owner.receive, p.owner.terminal = p.send, p.receive, p.terminal
	p.owner.ackWritten = p.owner.ackSerial
	p.owner = nil
}

// Discard erases a rejected candidate without removing current stream bytes.
func (p *Replacement) Discard() {
	if p.owner == nil {
		return
	}
	clear(p.send.data)
	p.owner = nil
}
