package admission

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuerprofile"
	"github.com/dianabuilds/ardents-network/internal/domain/network"
)

// AuthorityFacts are the independently authenticated facts supplied for one
// operation. Admission checks their binding; it cannot create Network authority.
// The provider must re-observe its authority at each operation boundary.
type AuthorityFacts struct {
	NetworkID, StateGeneration, StateDigest [32]byte
	Digest, IssuanceAuthorityKey            [32]byte
	IssuerNodeID                            [32]byte
	IssuerDutyGeneration                    uint64
	Epoch                                   uint64
	NotBefore, NotAfter                     time.Time
	TokenKeyCount                           uint8
	TokenKeys                               [18]TokenKey
}

// TokenKey is one exact class/hour public issuance key, copied by value.
type TokenKey struct {
	WindowStart time.Time
	Class       uint8
	SPKI        [issuerprofile.TokenSPKISize]byte
}

// ValidateAt checks consistency of supplied facts, never their authenticity.
func (p AuthorityFacts) ValidateAt(now time.Time) error {
	if now.IsZero() || p.NetworkID == [32]byte{} || p.StateGeneration == [32]byte{} || p.StateDigest == [32]byte{} ||
		p.Digest == [32]byte{} || p.IssuanceAuthorityKey == [32]byte{} || p.IssuerNodeID == [32]byte{} || p.IssuerDutyGeneration == 0 ||
		p.TokenKeyCount == 0 || int(p.TokenKeyCount) > len(p.TokenKeys) || now.Before(p.NotBefore) || !now.Before(p.NotAfter) {
		return errors.New("admission facts unavailable")
	}
	keys := make([]issuerprofile.Key, int(p.TokenKeyCount))
	for i, k := range p.TokenKeys[:p.TokenKeyCount] {
		keys[i] = issuerprofile.Key{Class: k.Class, Window: uint64(k.WindowStart.Unix()), SPKI: k.SPKI[:]}
	}
	return issuerprofile.ValidateCohorts(p.NotBefore, p.NotAfter, keys)
}

// FactsFromNetworkProfile copies the binding and public issuance inventory from
// one supplied Network profile. It authenticates nothing and grants no operation
// authority. Callers must observe the original State at each required effect.
// An oversized inventory yields zero, non-accepting facts.
func FactsFromNetworkProfile(profile network.ProfileFacts) AuthorityFacts {
	if len(profile.TokenKeys) > len((AuthorityFacts{}).TokenKeys) {
		return AuthorityFacts{}
	}
	authority := AuthorityFacts{
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
		authority.TokenKeys[i] = TokenKey{
			WindowStart: key.WindowStart,
			Class:       key.Class,
			SPKI:        key.SPKI,
		}
	}
	return authority
}
