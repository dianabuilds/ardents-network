package node

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// attachTransitGrantSigner verifies the credential-owned profile selected by
// State and projects only its purpose-scoped signing key to receiving duties.
// An absent or legacy v1 profile retains historical Epoch-authority Grant
// verification; a v2 profile never falls back to those authorities.
func attachTransitGrantSigner(snapshot *dutyFacts, now time.Time) error {
	if snapshot == nil || len(snapshot.TransitIssuerProfile) == 0 {
		return nil
	}
	profile, err := credential.DecodeProfile(snapshot.TransitIssuerProfile)
	if err != nil || profile.NodeID != snapshot.TransitIssuerNodeID {
		return errors.New("state transit issuance profile is invalid")
	}
	var issuer dutyCandidate
	found := false
	for index := uint8(0); index < snapshot.CandidateCount; index++ {
		candidate := snapshot.Candidates[index]
		if candidate.NodeID != snapshot.TransitIssuerNodeID {
			continue
		}
		if found || candidate.Assignment != "transit-issuance" || candidate.PublicKey == [32]byte{} {
			return errors.New("state transit issuance candidate is ambiguous")
		}
		issuer, found = candidate, true
	}
	if !found {
		return errors.New("state transit issuance candidate is absent")
	}
	deadline := now.Add(15 * time.Second)
	for _, bound := range []time.Time{snapshot.ValidUntil, issuer.ValidUntil, issuer.AssignmentNotAfter, profile.AssignmentNotAfter} {
		if !bound.IsZero() && bound.Before(deadline) {
			deadline = bound
		}
	}
	if !now.Before(deadline) || credential.VerifyProfile(profile, snapshot.NetworkID, issuer.NodeID, issuer.PublicKey, now, deadline) != nil {
		return errors.New("state transit issuance profile is not current")
	}
	if profile.Version >= 3 && !stateBindsIssuerInitiator(*snapshot, profile, now, deadline) {
		return errors.New("state transit issuance Initiator binding is invalid")
	}
	if profile.Version == 1 {
		return nil
	}
	snapshot.TransitGrantSignerID = profile.GrantSignerID
	snapshot.TransitGrantSignerPublicKey = profile.GrantSignerPublicKey
	return nil
}

func stateBindsIssuerInitiator(snapshot dutyFacts, profile credential.Profile, now, deadline time.Time) bool {
	found := false
	for index := uint8(0); index < snapshot.CandidateCount; index++ {
		candidate := snapshot.Candidates[index]
		if candidate.Assignment != "initiator" || candidate.NodeID != profile.InitiatorNodeID {
			continue
		}
		if found || candidate.PublicKey != profile.InitiatorPublicKey || candidate.NodeID == snapshot.TransitIssuerNodeID ||
			candidate.ValidFrom.After(now) || candidate.ValidUntil.Before(deadline) || candidate.AssignmentNotAfter.Before(deadline) {
			return false
		}
		found = true
	}
	return found
}
