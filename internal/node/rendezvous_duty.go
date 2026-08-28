package node

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func rendezvousDuty(profile RendezvousProfile, snapshot dutyFacts) (RendezvousConfig, error) {
	if snapshot.Profile != route.Profile || snapshot.Assignment != "rendezvous" || snapshot.ProbeEndpoint == "" ||
		profile.HandshakeLimit == 0 || profile.WaitingLimit == 0 || profile.PairLimit == 0 || profile.PairByteLimit == 0 ||
		!validAdmissionTimeout(profile.AdmissionTimeout) || profile.DrainTimeout <= 0 {
		return RendezvousConfig{}, errors.New("Rendezvous profile or State assignment is incomplete")
	}
	notAfter := snapshot.ValidUntil
	if snapshot.RecordValidUntil.Before(notAfter) {
		notAfter = snapshot.RecordValidUntil
	}
	peers := make([]RendezvousPeer, 0, 2)
	for index := uint8(0); index < snapshot.CandidateCount; index++ {
		candidate := snapshot.Candidates[index]
		if candidate.NodeID == snapshot.NodeID || candidate.NodeID == [32]byte{} || candidate.PublicKey == [32]byte{} {
			continue
		}
		role := byte(0)
		switch candidate.Assignment {
		case "initiator":
			role = route.InitiatorRole
		case "responder":
			role = route.ResponderRole
		default:
			continue
		}
		if candidate.ValidFrom.After(snapshot.EpochValidFrom) || candidate.ValidUntil.Before(notAfter) {
			return RendezvousConfig{}, errors.New("rendezvous peer is not valid for the complete duty")
		}
		peers = append(peers, RendezvousPeer{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, Role: role})
	}
	if len(peers) != 2 || peers[0].Role == peers[1].Role {
		return RendezvousConfig{}, errors.New("state does not supply one Initiator and one Responder peer")
	}
	listen, err := rendezvousListenAddress(snapshot.ProbeEndpoint, profile.LoopbackListenOverride)
	if err != nil {
		return RendezvousConfig{}, err
	}
	return RendezvousConfig{ListenAddress: listen, CarrierProfile: route.CarrierProfile(snapshot.CarrierProfile), Certificate: profile.Certificate,
		NetworkID: snapshot.NetworkID, EpochDigest: snapshot.Digest, NodeID: snapshot.NodeID,
		NodePublicKey: snapshot.NodePublicKey, Epoch: snapshot.Epoch, NotAfter: notAfter.UTC(), Peers: peers,
		HandshakeLimit: profile.HandshakeLimit, WaitingLimit: profile.WaitingLimit, PairLimit: profile.PairLimit,
		PairByteLimit: profile.PairByteLimit, AdmissionTimeout: profile.AdmissionTimeout, DrainTimeout: profile.DrainTimeout}, nil
}
