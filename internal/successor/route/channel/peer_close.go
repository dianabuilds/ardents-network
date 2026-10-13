package channel

import "errors"

// PeerClose is evidence that the original dedicated child reader decoded
// CLOSE(0). It grants no live channel, Service completion or cleanup success.
type PeerClose struct{ lane *Lane }

// DecodedPeerClose observes only the actual reader's canonical CLOSE state.
// Local Close, EOF, cancellation and a peer refusal cannot create evidence.
func (lane *Lane) DecodedPeerClose() (PeerClose, error) {
	if lane == nil || lane.s == nil {
		return PeerClose{}, errors.New("route peer CLOSE evidence absent")
	}
	lane.s.mu.Lock()
	defer lane.s.mu.Unlock()
	if !lane.s.dedicated || lane.s.lanes[lane.id] != lane || !lane.peerClosed || lane.peerRefused {
		return PeerClose{}, errors.New("route peer CLOSE evidence absent")
	}
	return PeerClose{lane: lane}, nil
}

// CheckChild prevents another child or a zero/copy-constructed lane from
// borrowing this decoded fact. Subsequent cleanup remains separately owned.
func (proof PeerClose) CheckChild(lane *Lane) error {
	if lane == nil || proof.lane != lane {
		return errors.New("route peer CLOSE evidence belongs to another child")
	}
	return nil // The opaque fact remains bound after physical retirement.
}
