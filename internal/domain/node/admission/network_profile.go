package admission

import (
	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/network"
)

// AuthorityFactsFromProfile copies the binding and public issuance inventory from
// one Network observation. The caller is responsible for observing the
// original State again at each required effect.
func AuthorityFactsFromProfile(profile network.ProfileFacts) admission.AuthorityFacts {
	authority := admission.AuthorityFacts{
		NetworkID:            profile.Network,
		StateGeneration:      profile.Generation,
		StateDigest:          profile.EpochDigest,
		Digest:               profile.Digest,
		IssuanceAuthorityKey: profile.IssuanceAuthorityKey,
		IssuerNodeID:         profile.IssuerNodeID,
		IssuerDutyGeneration: profile.IssuerDutyGeneration,
		Epoch:                profile.Epoch,
		NotBefore:            profile.NotBefore,
		NotAfter:             profile.NotAfter,
		TokenKeyCount:        uint8(len(profile.TokenKeys)),
	}
	for i, key := range profile.TokenKeys {
		authority.TokenKeys[i] = admission.TokenKey{
			WindowStart: key.WindowStart,
			Class:       key.Class,
			SPKI:        key.SPKI,
		}
	}
	return authority
}
