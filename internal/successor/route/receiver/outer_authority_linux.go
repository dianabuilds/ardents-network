package receiver

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// Each call obtains fresh original authority for this physical frame.
func (r *Receiver) checkOuterPeer(h ardp.Hello, member, peer network.Member, peerKey [32]byte) error {
	var current network.Member
	var err error
	if member.RoleDomain == 2 && (member.Subrole == 4 || member.Subrole == 5) && r.registrations == nil {
		// Neither parent check observes Registry or Store between local and peer
		// validation. Match HELLO and the original peer in one fresh owner view,
		// then repeat the full observation at the next framing boundary.
		// Descriptor's separate Store checks still bracket its role effects.
		var view network.RuntimeView
		var local network.Member
		view, local, err = r.config.Authority.ObserveHello(h, true)
		if err == nil {
			current, err = adjacentPeer(view, local, peerKey)
		}
	} else {
		// Introduction's separate native Registry bracket and other existing role
		// paths keep their independently owned observations unchanged.
		if _, err = r.config.Authority.Hello(h, true); err != nil {
			return err
		}
		current, err = r.peer(peerKey)
	}
	if err != nil || current.RecordDigest != peer.RecordDigest || current.DutyGeneration != peer.DutyGeneration {
		return errors.Join(errors.New("route Node peer changed"), err)
	}
	return nil
}
