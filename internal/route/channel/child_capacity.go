package channel

import "github.com/dianabuilds/ardents-network/internal/route/ardp"

const (
	workChildren    = uint32(256)
	controlChildren = uint32(2)
)

// newChildLocked claims one original lane under both the session and shared
// principal bounds. Controls use ordinary capacity first; only the four named
// purposes may borrow the extra reserve. The claim is fixed until physical
// Finish, even if an ordinary slot becomes available in the meantime.
func (s *Session) newChildLocked(id uint32, body []byte) *Lane {
	reserved := s.live-s.reservedControls >= workChildren
	if reserved && (s.reservedControls >= controlChildren || !reservedControlOpen(body)) {
		return nil
	}
	if !s.queues.child() {
		return nil
	}
	l := s.newLaneLocked(id)
	l.reservedControl = reserved
	if reserved {
		s.reservedControls++
	}
	return l
}

// The framing reader delegates ordinary OPEN semantic checks to its handler.
// An extra control claim additionally requires canonical recipient grammar;
// neither its public purpose nor this capacity claim grants role authority.
func reservedControlOpen(body []byte) bool {
	var opened ardp.Open
	var err error
	if len(body) == 50 {
		var envelope ardp.NodeOpen
		envelope, err = ardp.DecodeNodeOpen(body)
		if err != nil {
			return false
		}
		opened = envelope.Recipient
	} else {
		opened, err = ardp.DecodeOpen(body)
		if err != nil {
			return false
		}
	}
	switch ardp.Purpose(opened.Purpose) {
	case ardp.PurposeIssuer, ardp.PurposeReachability, ardp.PurposeIntroduction, ardp.PurposeSubmission:
		return true
	}
	return false
}

// finishChildLocked is shared by original Lane Finish and joined parent Close.
// A peer CLOSE or failed setup cannot return a slot before its owner joins.
func (s *Session) finishChildLocked(l *Lane) {
	if l.finished {
		return
	}
	l.finished = true
	s.live--
	if l.reservedControl {
		s.reservedControls--
	}
	s.queues.releaseChild()
}
