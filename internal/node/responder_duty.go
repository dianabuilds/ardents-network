package node

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func responderDuty(profile ResponderProfile, snapshot dutyFacts, admit route.EndpointTransitBindingAdmitter) (responderConfig, error) {
	if snapshot.Profile != route.Profile || snapshot.Assignment != "responder" || snapshot.ProbeEndpoint == "" || admit == nil ||
		profile.HandshakeLimit == 0 || profile.RelayLimit == 0 || profile.RelayByteLimit == 0 || !validAdmissionTimeout(profile.AdmissionTimeout) || profile.DrainTimeout <= 0 {
		return responderConfig{}, errors.New("responder profile or State assignment is incomplete")
	}
	notAfter := snapshot.ValidUntil
	if snapshot.RecordValidUntil.Before(notAfter) {
		notAfter = snapshot.RecordValidUntil
	}
	var peer responderPeer
	for index := uint8(0); index < snapshot.CandidateCount; index++ {
		candidate := snapshot.Candidates[index]
		if candidate.Assignment != "rendezvous" || candidate.NodeID == [32]byte{} || candidate.PublicKey == [32]byte{} || candidate.NodeID == snapshot.NodeID {
			continue
		}
		if candidate.ValidFrom.After(snapshot.EpochValidFrom) || candidate.ValidUntil.Before(notAfter) || peer.NodeID != [32]byte{} {
			return responderConfig{}, errors.New("responder State Rendezvous peer is incomplete or not valid for the duty")
		}
		peer = responderPeer{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, Endpoint: candidate.Endpoint,
			CarrierProfile: route.CarrierProfile(candidate.CarrierProfile)}
	}
	if peer.NodeID == [32]byte{} {
		return responderConfig{}, errors.New("responder State supplies no Rendezvous peer")
	}
	return responderConfig{ListenAddress: snapshot.ProbeEndpoint, Certificate: profile.Certificate, NetworkID: snapshot.NetworkID, EpochDigest: snapshot.Digest,
		NodeID: snapshot.NodeID, NodePublicKey: snapshot.NodePublicKey, Epoch: snapshot.Epoch, NotAfter: notAfter.UTC(), rendezvous: peer,
		Admit: admit, HandshakeLimit: profile.HandshakeLimit, RelayLimit: profile.RelayLimit, RelayByteLimit: profile.RelayByteLimit, AdmissionTimeout: profile.AdmissionTimeout, DrainTimeout: profile.DrainTimeout}, nil
}
