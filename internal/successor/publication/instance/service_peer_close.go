package instance

import "github.com/dianabuilds/ardents-network/internal/successor/route/join"

// DecodedPeerClose projects the exact admitted JOIN's terminal evidence.
// Connection still checks its original logical authority and Terminal ledger;
// Instance supplies no replacement or success permission through this fact.
func (service *ServiceAttachment) DecodedPeerClose() (join.PeerClose, error) {
	if service == nil || service.self != service || service.physical == nil {
		return join.PeerClose{}, ErrUnavailable
	}
	proof, err := service.physical.DecodedPeerClose()
	if err == nil {
		err = proof.CheckOriginal(service.physical)
	}
	return proof, err
}
