package node

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func introductionDuty(profile IntroductionProfile, snapshot dutyFacts, admit route.EndpointTransitBindingAdmitter) (introductionConfig, error) {
	if snapshot.Profile != route.Profile || snapshot.Assignment != "introduction" || snapshot.ProbeEndpoint == "" || admit == nil ||
		profile.HandshakeLimit == 0 || profile.SlotLimit == 0 || profile.DeliveryLimit == 0 || !validAdmissionTimeout(profile.AdmissionTimeout) || profile.DrainTimeout <= 0 {
		return introductionConfig{}, errors.New("introduction profile or State assignment is incomplete")
	}
	notAfter := snapshot.ValidUntil
	if snapshot.RecordValidUntil.Before(notAfter) {
		notAfter = snapshot.RecordValidUntil
	}
	return introductionConfig{ListenAddress: snapshot.ProbeEndpoint, Certificate: profile.Certificate, NetworkID: snapshot.NetworkID,
		EpochDigest: snapshot.Digest, NodeID: snapshot.NodeID, NodePublicKey: snapshot.NodePublicKey, Epoch: snapshot.Epoch,
		NotAfter: notAfter.UTC(), Admit: admit, HandshakeLimit: profile.HandshakeLimit, SlotLimit: profile.SlotLimit,
		DeliveryLimit: profile.DeliveryLimit, AdmissionTimeout: profile.AdmissionTimeout, DrainTimeout: profile.DrainTimeout}, nil
}
