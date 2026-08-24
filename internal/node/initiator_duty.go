package node

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func initiatorDuty(profile InitiatorProfile, snapshot dutyFacts) (InitiatorConfig, error) {
	if snapshot.Profile != route.Profile || snapshot.Assignment != "initiator" || snapshot.ProbeEndpoint == "" || profile.Admit == nil ||
		profile.HandshakeLimit == 0 || profile.RelayLimit == 0 || profile.RelayByteLimit == 0 || profile.DrainTimeout <= 0 {
		return InitiatorConfig{}, errors.New("Initiator profile or State assignment is incomplete")
	}
	notAfter := snapshot.ValidUntil
	if snapshot.RecordValidUntil.Before(notAfter) {
		notAfter = snapshot.RecordValidUntil
	}
	var peer InitiatorPeer
	for index := uint8(0); index < snapshot.CandidateCount; index++ {
		candidate := snapshot.Candidates[index]
		if candidate.Assignment != "rendezvous" || candidate.NodeID == [32]byte{} || candidate.PublicKey == [32]byte{} ||
			candidate.NodeID == snapshot.NodeID {
			continue
		}
		if candidate.ValidFrom.After(snapshot.EpochValidFrom) || candidate.ValidUntil.Before(notAfter) || peer.NodeID != [32]byte{} {
			return InitiatorConfig{}, errors.New("Initiator State Rendezvous peer is incomplete or not valid for the duty")
		}
		peer = InitiatorPeer{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, Endpoint: candidate.Endpoint}
	}
	if peer.NodeID == [32]byte{} {
		return InitiatorConfig{}, errors.New("Initiator State supplies no Rendezvous peer")
	}
	return InitiatorConfig{ListenAddress: snapshot.ProbeEndpoint, Certificate: profile.Certificate, NetworkID: snapshot.NetworkID,
		EpochDigest: snapshot.Digest, NodeID: snapshot.NodeID, NodePublicKey: snapshot.NodePublicKey, Epoch: snapshot.Epoch,
		NotAfter: notAfter.UTC(), Rendezvous: peer, Admit: profile.Admit, HandshakeLimit: profile.HandshakeLimit,
		RelayLimit: profile.RelayLimit, RelayByteLimit: profile.RelayByteLimit, DrainTimeout: profile.DrainTimeout}, nil
}

func validateNativeDutyProfile(config runtimeConfig, snapshot dutyFacts) error {
	switch snapshot.Assignment {
	case "rendezvous":
		_, err := rendezvousDuty(config.Rendezvous, snapshot)
		return err
	case "initiator":
		_, err := initiatorDuty(config.Initiator, snapshot)
		return err
	default:
		return errors.New("native Route assignment is not implemented")
	}
}
