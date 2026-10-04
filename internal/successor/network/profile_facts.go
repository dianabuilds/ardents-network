package network

import "time"

// TokenKey is copied public inventory from an authenticated role profile.
// Admission's format owner validates its canonical SPKI and hour cohorts;
// Network retains the inventory under the accepted profile identity.
type TokenKey struct {
	WindowStart time.Time
	Class       uint8
	SPKI        [346]byte
}

// ProfileFacts binds the public issuance inventory to the same generation as
// membership. These supplied facts contain no authentication capability or
// private signing material. Only the opened application can authenticate and
// commit their source before constructing a runtime observation.
type ProfileFacts struct {
	ProfileBinding
	IssuanceAuthorityKey, IssuerNodeID [32]byte
	IssuerDutyGeneration               uint64
	TokenKeys                          []TokenKey
}

func (profile ProfileFacts) copied() ProfileFacts {
	profile.TokenKeys = append([]TokenKey(nil), profile.TokenKeys...)
	return profile
}

func (profile ProfileFacts) matchesIssuer(membership Membership) bool {
	index, exists := membership.byNode[profile.IssuerNodeID]
	return profile.IssuanceAuthorityKey != [32]byte{} && profile.IssuerNodeID != [32]byte{} &&
		profile.IssuerDutyGeneration != 0 && len(profile.TokenKeys) > 0 && len(profile.TokenKeys) <= 18 &&
		exists && index >= 0 && membership.members[index].DutyGeneration == profile.IssuerDutyGeneration
}
