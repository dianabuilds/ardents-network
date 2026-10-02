package admission

import (
	"crypto/sha256"
	"math"
	"time"
)

// TokenKey is one exact public class/hour key, never a signing key.
type TokenKey struct {
	Window uint64
	Class  uint8
	SPKI   []byte
}

// LedgerBinding is explicitly selected offline evidence, not authenticated live State.
type LedgerBinding struct {
	Network, Issuer, Authority, Profile [32]byte
	Duty                                uint64
	Start, End                          time.Time
	Keys                                []TokenKey
}

func (b LedgerBinding) valid() bool {
	zero := [32]byte{}
	start, end := b.Start.Unix(), b.End.Unix()
	if b.Network == zero || b.Issuer == zero || b.Authority == zero || b.Profile == zero || b.Duty == 0 || start < 0 || start > math.MaxInt64-21600 || start%3600 != 0 || end <= start || end-start > 21600 || end%3600 != 0 || b.Start.Nanosecond() != 0 || b.End.Nanosecond() != 0 || len(b.Keys) != int((end-start)/3600)*3 {
		return false
	}
	seen := map[[32]byte]bool{}
	for i, key := range b.Keys {
		if key.Window != uint64(start)+uint64(i/3)*3600 || key.Class != uint8(i%3+1) {
			return false
		}
		if _, ok := tokenKey(key.SPKI); !ok {
			return false
		}
		// A dedicated key cannot be reused across class/hour cohorts.
		id := sha256.Sum256(key.SPKI)
		if seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

func (b LedgerBinding) matches(f Facts) bool {
	return b.Network == f.Network && b.Issuer == f.Issuer && b.Authority == f.Authority && b.Duty == f.Duty && b.Start.Equal(f.DutyNotBefore) && b.End.Equal(f.DutyNotAfter)
}
