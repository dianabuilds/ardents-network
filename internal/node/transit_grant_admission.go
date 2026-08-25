package node

import (
	"crypto/ed25519"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// stateTransitGrantAdmitter verifies and durably consumes only a current
// State-authorized exact C-2 transit grant. It is the Node-owned completion of
// Route's opaque EndpointTransitBinding admission port.
func stateTransitGrantAdmitter(root string, snapshot dutyFacts, now func() time.Time) route.EndpointTransitBindingAdmitter {
	return func(raw []byte, attachment, clientKey [32]byte, role byte, transitNode [32]byte, notAfter time.Time) (route.EndpointTransitAdmission, error) {
		if root == "" || now == nil || snapshot.AuthorityCount == 0 || role != snapshotTransitRole(snapshot) || transitNode != snapshot.NodeID {
			return route.EndpointTransitAdmission{}, errors.New("state transit grant admission is incomplete")
		}
		unverified, err := route.DecodeTransitGrant(raw)
		if err != nil {
			return route.EndpointTransitAdmission{}, err
		}
		var authority ed25519.PublicKey
		for index := uint8(0); index < snapshot.AuthorityCount; index++ {
			candidate := snapshot.Authorities[index]
			if candidate.ID == unverified.IssuerID {
				authority = ed25519.PublicKey(candidate.PublicKey[:])
				break
			}
		}
		grant, err := route.VerifyTransitGrant(raw, authority)
		if err != nil || grant.NetworkID != snapshot.NetworkID || grant.Digest != snapshot.Digest || grant.Epoch != snapshot.Epoch ||
			grant.AttachmentID != attachment || grant.TransitRole != role || grant.TransitNodeID != transitNode ||
			grant.ClientKeyDigest != clientKey || !grant.NotAfter.Equal(notAfter) || !now().UTC().Before(grant.NotAfter) {
			return route.EndpointTransitAdmission{}, errors.New("state transit grant does not match current duty")
		}
		ledger, err := duty.Open(duty.Config{Root: root, Clock: now, Create: true})
		if err != nil {
			return route.EndpointTransitAdmission{}, err
		}
		err = errors.Join(ledger.SpendTransitGrant(snapshot.NodeID, grant.GrantID, grant.NotAfter), ledger.Close())
		if err != nil {
			return route.EndpointTransitAdmission{}, err
		}
		return route.EndpointTransitAdmission{AuthorizationID: grant.GrantID, NetworkID: grant.NetworkID, Digest: grant.Digest,
			Epoch: grant.Epoch, TransitRole: grant.TransitRole, TransitNodeID: grant.TransitNodeID, NotAfter: grant.NotAfter}, nil
	}
}

func snapshotTransitRole(snapshot dutyFacts) byte {
	if snapshot.Assignment == "introduction" {
		return route.IntroductionRole
	}
	if snapshot.Assignment == "responder" {
		return route.ResponderRole
	}
	return 0
}
