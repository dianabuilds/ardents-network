package admission

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"
	"time"
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
	SPKI        [346]byte
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
