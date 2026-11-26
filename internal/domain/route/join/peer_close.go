package join

import (
	"errors"

	framing "github.com/dianabuilds/ardents-network/internal/domain/route/channel"
)

// PeerClose retains the decoded clean terminal of one exact original JOIN.
// It attests neither Service Terminal authentication nor physical cleanup.
type PeerClose struct {
	stream *Joined
	child  framing.PeerClose
}

func (j *Joined) DecodedPeerClose() (PeerClose, error) {
	if j == nil || j.acquisition == nil || j.session == nil || j.lane == nil {
		return PeerClose{}, errors.New("original JOIN peer CLOSE evidence absent")
	}
	j.acquisition.mu.Lock()
	original := j.acquisition.stream == j
	j.acquisition.mu.Unlock()
	if !original {
		return PeerClose{}, errors.New("original JOIN peer CLOSE evidence absent")
	}
	child, err := j.lane.DecodedPeerClose()
	if err != nil {
		return PeerClose{}, err
	}
	return PeerClose{stream: j, child: child}, nil
}

func (proof PeerClose) CheckOriginal(joined *Joined) error {
	if joined == nil || proof.stream != joined {
		return errors.New("JOIN peer CLOSE evidence belongs to another stream")
	}
	return proof.child.CheckChild(joined.lane)
}
