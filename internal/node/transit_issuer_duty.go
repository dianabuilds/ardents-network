package node

import (
	"crypto/sha256"
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func transitIssuerStateDuty(snapshot dutyFacts, now time.Time) (credential.StateDuty, bool) {
	if snapshot.Assignment != "transit-issuance" || snapshot.NodeID == [32]byte{} || snapshot.NodePublicKey == [32]byte{} ||
		snapshot.TransitIssuerNodeID != snapshot.NodeID || len(snapshot.TransitIssuerProfile) == 0 || !now.Before(snapshot.ValidUntil) ||
		!now.Before(snapshot.RecordValidUntil) {
		return credential.StateDuty{}, false
	}
	profile, err := credential.DecodeProfile(snapshot.TransitIssuerProfile)
	if err != nil || profile.Version < 3 || profile.NetworkID != snapshot.NetworkID || profile.NodeID != snapshot.NodeID ||
		profile.AssignmentNotAfter.After(snapshot.ValidUntil) || profile.AssignmentNotAfter.After(snapshot.RecordValidUntil) ||
		!now.Before(profile.AssignmentNotAfter) {
		return credential.StateDuty{}, false
	}
	issuerSeen, issuerBound, initiatorSeen, initiatorBound := false, false, false, false
	for index := uint8(0); index < snapshot.CandidateCount; index++ {
		candidate := snapshot.Candidates[index]
		if candidate.NodeID == snapshot.NodeID && candidate.Assignment == "transit-issuance" {
			if issuerSeen {
				return credential.StateDuty{}, false
			}
			issuerSeen = true
			issuerBound = candidate.PublicKey == snapshot.NodePublicKey &&
				!candidate.ValidFrom.After(now) && !candidate.ValidUntil.Before(profile.AssignmentNotAfter) &&
				!candidate.AssignmentNotAfter.Before(profile.AssignmentNotAfter)
		}
		if candidate.NodeID == profile.InitiatorNodeID && candidate.Assignment == "initiator" {
			if initiatorSeen {
				return credential.StateDuty{}, false
			}
			initiatorSeen = true
			initiatorBound = candidate.PublicKey == profile.InitiatorPublicKey &&
				!candidate.ValidFrom.After(now) && !candidate.ValidUntil.Before(profile.AssignmentNotAfter) &&
				!candidate.AssignmentNotAfter.Before(profile.AssignmentNotAfter)
		}
	}
	if !issuerBound || !initiatorBound {
		return credential.StateDuty{}, false
	}
	return credential.StateDuty{NetworkID: snapshot.NetworkID, Digest: snapshot.Digest, IssuerNodeID: snapshot.NodeID,
		IssuerPublicKey: snapshot.NodePublicKey, InitiatorNodeID: profile.InitiatorNodeID, InitiatorPublicKey: profile.InitiatorPublicKey,
		GrantSignerPublicKey: profile.GrantSignerPublicKey, ProfileDigest: sha256.Sum256(snapshot.TransitIssuerProfile),
		Epoch: snapshot.Epoch, NotAfter: profile.AssignmentNotAfter}, true
}

func validateTransitIssuerProfile(local TransitIssuerProfile, snapshot dutyFacts, now time.Time) error {
	if local.Root == "" || !filepath.IsAbs(local.Root) || filepath.Clean(local.Root) != local.Root ||
		local.Certificate.PrivateKey == nil || local.ConnectionLimit == 0 || local.ConnectionLimit > 64 ||
		local.DrainTimeout <= 0 || local.DrainTimeout > time.Minute || !literalNodeEndpoint(snapshot.ProbeEndpoint) {
		return errors.New("Transit Grant issuer local profile is incomplete")
	}
	if _, available := transitIssuerStateDuty(snapshot, now); !available {
		return errors.New("Transit Grant issuer State duty is unavailable")
	}
	return nil
}
