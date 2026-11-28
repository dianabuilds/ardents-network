package transfer

import "errors"

// ErrSegmentDraining means new local input must wait for the next segment.
var ErrSegmentDraining = errors.New("service Connection segment is draining")

// SegmentPositions contains the original segment's physically accounted local
// positions and confirmed Application EOF. It grants no next-Connection effect.
type SegmentPositions struct {
	Sent, Received       uint64
	SentEOF, ReceivedEOF bool
}

// FreezeSegment stops newly admitted sends without ending the Application
// direction. Previously accepted sends and incoming bytes still drain.
// The Stream serializes this operation with all ordinary ledger transitions.
func (s *State) FreezeSegment() { s.draining = true }

// DrainedSegment reports a candidate boundary after existing physical writes,
// peer acknowledgements and every local byte delivery have caught up. The
// endpoint still needs the peer's exact boundary confirmation and physical join.
func (s *State) DrainedSegment() (SegmentPositions, bool) {
	if !s.draining || s.send.active != nil || s.terminal.active != nil ||
		s.send.next != s.send.end || s.send.base != s.send.end ||
		s.receive.readAt != s.receive.next || s.ackWritten != s.ackSerial {
		return SegmentPositions{}, false
	}
	terminal := s.terminal
	if s.send.closed && (!terminal.terminalWritten || !terminal.receiptReceived || !terminal.confirmationWritten) {
		return SegmentPositions{}, false
	}
	if s.receive.terminal && (!terminal.remoteTerminalCurrent || !terminal.receiptWritten || !terminal.receiptConfirmed) {
		return SegmentPositions{}, false
	}
	return SegmentPositions{
		Sent: s.send.end, Received: s.receive.readAt,
		SentEOF: s.send.closed, ReceivedEOF: s.receive.terminal,
	}, true
}
