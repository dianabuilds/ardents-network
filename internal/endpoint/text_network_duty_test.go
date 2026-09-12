//go:build linux

package endpoint

import (
	"crypto/sha256"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// This explicit public-State fixture feeds Node's lifecycle interface for the
// Endpoint network test. It provides no State acceptance or host qualification.
// Every fact is projected from the same snapshot used by Endpoint selection.
type textNetworkDutyFixture struct{ snapshot state.Snapshot }

func (view textNetworkDutyFixture) DutyGeneration() string  { return view.snapshot.Generation }
func (view textNetworkDutyFixture) DutyNetworkID() [32]byte { return view.snapshot.NetworkID }
func (view textNetworkDutyFixture) DutyEpoch() uint64       { return view.snapshot.Epoch }
func (view textNetworkDutyFixture) DutyDigest() [32]byte    { return view.snapshot.Digest }
func (view textNetworkDutyFixture) DutyEpochValidFrom() time.Time {
	return view.snapshot.EpochValidFrom
}
func (view textNetworkDutyFixture) DutyValidUntil() time.Time   { return view.snapshot.ValidUntil }
func (view textNetworkDutyFixture) DutyProfile() string         { return view.snapshot.Profile }
func (view textNetworkDutyFixture) DutyFresh() bool             { return view.snapshot.Freshness == "fresh" }
func (view textNetworkDutyFixture) DutyConflicting() bool       { return view.snapshot.Conflicting }
func (view textNetworkDutyFixture) DutyRecordPresent() bool     { return view.snapshot.RecordPresent }
func (view textNetworkDutyFixture) DutyNodeID() [32]byte        { return view.snapshot.NodeID }
func (view textNetworkDutyFixture) DutyNodePublicKey() [32]byte { return view.snapshot.NodePublicKey }
func (view textNetworkDutyFixture) DutyRecordGeneration() uint64 {
	return view.snapshot.RecordGeneration
}
func (view textNetworkDutyFixture) DutyRecordValidFrom() time.Time {
	return view.snapshot.RecordValidFrom
}
func (view textNetworkDutyFixture) DutyRecordValidUntil() time.Time {
	return view.snapshot.RecordValidUntil
}
func (view textNetworkDutyFixture) DutyDeclaredFamily() string { return view.snapshot.DeclaredFamily }
func (view textNetworkDutyFixture) DutyProbeEndpoint() string  { return view.snapshot.ProbeEndpoint }
func (view textNetworkDutyFixture) DutyCarrierProfile() string { return view.snapshot.CarrierProfile }
func (view textNetworkDutyFixture) DutyProbeCapacity() uint16  { return view.snapshot.ProbeCapacity }
func (view textNetworkDutyFixture) DutyAssignment() string     { return view.snapshot.Assignment }
func (view textNetworkDutyFixture) DutyAssignmentDigest() [32]byte {
	return view.snapshot.AssignmentDigest
}
func (view textNetworkDutyFixture) DutyCandidateCount() uint8 { return view.snapshot.CandidateCount }
func (view textNetworkDutyFixture) DutyCandidateNodeID(index uint8) [32]byte {
	if index >= view.snapshot.CandidateCount {
		return [32]byte{}
	}
	return view.snapshot.Candidates[index].NodeID
}
func (view textNetworkDutyFixture) DutyCandidatePublicKey(index uint8) [32]byte {
	if index >= view.snapshot.CandidateCount {
		return [32]byte{}
	}
	return view.snapshot.Candidates[index].PublicKey
}
func (view textNetworkDutyFixture) DutyCandidateKeyID(index uint8) [32]byte {
	if index >= view.snapshot.CandidateCount {
		return [32]byte{}
	}
	return view.snapshot.Candidates[index].KeyID
}
func (view textNetworkDutyFixture) DutyCandidateFamilyID(index uint8) [32]byte {
	if index >= view.snapshot.CandidateCount {
		return [32]byte{}
	}
	return view.snapshot.Candidates[index].FamilyID
}
func (view textNetworkDutyFixture) DutyCandidateRecordDigest(index uint8) [32]byte {
	if index >= view.snapshot.CandidateCount {
		return [32]byte{}
	}
	return view.snapshot.Candidates[index].RecordDigest
}
func (view textNetworkDutyFixture) DutyCandidateDomainProofDigest(index uint8) [32]byte {
	if index >= view.snapshot.CandidateCount {
		return [32]byte{}
	}
	return view.snapshot.Candidates[index].DomainProofDigest
}
func (view textNetworkDutyFixture) DutyCandidateEndpoint(index uint8) string {
	if index >= view.snapshot.CandidateCount {
		return ""
	}
	return view.snapshot.Candidates[index].Endpoint
}
func (view textNetworkDutyFixture) DutyCandidateCarrierProfile(index uint8) string {
	if index >= view.snapshot.CandidateCount {
		return ""
	}
	return view.snapshot.Candidates[index].CarrierProfile
}
func (view textNetworkDutyFixture) DutyCandidateCapacity(index uint8) uint16 {
	if index >= view.snapshot.CandidateCount {
		return 0
	}
	return view.snapshot.Candidates[index].Capacity
}
func (view textNetworkDutyFixture) DutyCandidateAssignment(index uint8) string {
	if index >= view.snapshot.CandidateCount {
		return ""
	}
	return view.snapshot.Candidates[index].Domain
}
func (view textNetworkDutyFixture) DutyCandidateValidFrom(index uint8) time.Time {
	if index >= view.snapshot.CandidateCount {
		return time.Time{}
	}
	return view.snapshot.Candidates[index].ValidFrom
}
func (view textNetworkDutyFixture) DutyCandidateValidUntil(index uint8) time.Time {
	if index >= view.snapshot.CandidateCount {
		return time.Time{}
	}
	return view.snapshot.Candidates[index].ValidUntil
}
func (view textNetworkDutyFixture) DutyCandidateAssignmentNotAfter(index uint8) time.Time {
	if index >= view.snapshot.CandidateCount {
		return time.Time{}
	}
	return view.snapshot.Candidates[index].AssignmentNotAfter
}

// DutyTransitIssuanceProfileDigest returns the exact authenticated issuer
// profile commitment for an Initiator's Credential Relay duty. It exposes no
// profile bytes or alternate issuer to the Node runtime.
func (view textNetworkDutyFixture) DutyTransitIssuanceProfileDigest() [32]byte {
	if view.snapshot.TransitIssuanceProfileSize == 0 || int(view.snapshot.TransitIssuanceProfileSize) > len(view.snapshot.TransitIssuanceProfile) {
		return [32]byte{}
	}
	return sha256.Sum256(view.snapshot.TransitIssuanceProfile[:view.snapshot.TransitIssuanceProfileSize])
}

// DutyTransitIssuanceNodeID and DutyTransitIssuanceProfile expose one
// authenticated opaque issuer declaration. Node passes it only to the
// credential owner; State does not parse that profile grammar.
func (view textNetworkDutyFixture) DutyTransitIssuanceNodeID() [32]byte {
	return view.snapshot.TransitIssuanceNodeID
}

func (view textNetworkDutyFixture) DutyTransitIssuanceProfile() []byte {
	if view.snapshot.TransitIssuanceProfileSize == 0 || int(view.snapshot.TransitIssuanceProfileSize) > len(view.snapshot.TransitIssuanceProfile) {
		return nil
	}
	return append([]byte(nil), view.snapshot.TransitIssuanceProfile[:view.snapshot.TransitIssuanceProfileSize]...)
}

// DutyAuthorityCount, DutyAuthorityID, and DutyAuthorityPublicKey expose the
// finite current Epoch authority set used only by historical Grant profiles.
// Current dynamic issuance projects its distinct Grant key from the opaque
// issuer profile. These operations expose no State source or persistence.
func (view textNetworkDutyFixture) DutyAuthorityCount() uint8 {
	return view.snapshot.EpochAuthorityCount
}
func (view textNetworkDutyFixture) DutyAuthorityID(index uint8) [32]byte {
	if index >= view.snapshot.EpochAuthorityCount {
		return [32]byte{}
	}
	return view.snapshot.EpochAuthorityIDs[index]
}
func (view textNetworkDutyFixture) DutyAuthorityPublicKey(index uint8) [32]byte {
	if index >= view.snapshot.EpochAuthorityCount {
		return [32]byte{}
	}
	return view.snapshot.EpochAuthorityKeys[index]
}
