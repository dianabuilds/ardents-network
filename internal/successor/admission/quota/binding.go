package quota

import "github.com/dianabuilds/ardents-network/internal/successor/admission"

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"
)

// LedgerBinding is explicitly selected offline evidence, not authenticated live State.
type LedgerBinding struct {
	Network, Issuer, Authority, Profile [32]byte
	Duty                                uint64
	Start, End                          time.Time
	Keys                                []issuerprofile.Key
}

func (b LedgerBinding) valid() bool {
	zero := [32]byte{}
	return b.Network != zero && b.Issuer != zero && b.Authority != zero && b.Profile != zero && b.Duty != 0 && issuerprofile.ValidateCohorts(b.Start, b.End, b.Keys) == nil
}

func (b LedgerBinding) matches(f admission.Facts) bool {
	return b.Network == f.Network && b.Issuer == f.Issuer && b.Authority == f.Authority && b.Duty == f.Duty && b.Start.Equal(f.DutyNotBefore) && b.End.Equal(f.DutyNotAfter)
}
