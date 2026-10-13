package join

import (
	"errors"

	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
)

// PeerClose retains the decoded clean terminal of one exact original JOIN.
// It attests neither Service Terminal authentication nor physical cleanup.
type PeerClose struct {
	stream *Joined
	child  framing.PeerClose
}

func (joined *Joined) DecodedPeerClose() (PeerClose, error) {
	if joined == nil || joined.acquisition == nil || joined.session == nil || joined.lane == nil {
		return PeerClose{}, errors.New("original JOIN peer CLOSE evidence absent")
	}
	joined.acquisition.mu.Lock()
	original := joined.acquisition.stream == joined
	joined.acquisition.mu.Unlock()
	if !original {
		return PeerClose{}, errors.New("original JOIN peer CLOSE evidence absent")
	}
	child, err := joined.lane.DecodedPeerClose()
	if err != nil {
		return PeerClose{}, err
	}
	return PeerClose{stream: joined, child: child}, nil
}

func (proof PeerClose) CheckOriginal(joined *Joined) error {
	if joined == nil || proof.stream != joined {
		return errors.New("JOIN peer CLOSE evidence belongs to another stream")
	}
	return proof.child.CheckChild(joined.lane)
}
