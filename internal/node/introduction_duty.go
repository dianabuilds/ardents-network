package node

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func introductionDuty(profile IntroductionProfile, snapshot dutyFacts) (IntroductionConfig, error) {
	if snapshot.Profile != route.Profile || snapshot.Assignment != "introduction" || snapshot.ProbeEndpoint == "" || profile.Admit == nil ||
		profile.HandshakeLimit == 0 || profile.SlotLimit == 0 || profile.DeliveryLimit == 0 || profile.DrainTimeout <= 0 {
		return IntroductionConfig{}, errors.New("Introduction profile or State assignment is incomplete")
	}
	notAfter := snapshot.ValidUntil
	if snapshot.RecordValidUntil.Before(notAfter) {
		notAfter = snapshot.RecordValidUntil
	}
	return IntroductionConfig{ListenAddress: snapshot.ProbeEndpoint, Certificate: profile.Certificate, NetworkID: snapshot.NetworkID,
		EpochDigest: snapshot.Digest, NodeID: snapshot.NodeID, NodePublicKey: snapshot.NodePublicKey, Epoch: snapshot.Epoch,
		NotAfter: notAfter.UTC(), Admit: profile.Admit, HandshakeLimit: profile.HandshakeLimit, SlotLimit: profile.SlotLimit,
		DeliveryLimit: profile.DeliveryLimit, DrainTimeout: profile.DrainTimeout}, nil
}
