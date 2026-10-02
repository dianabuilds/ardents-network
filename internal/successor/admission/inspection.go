package admission

import (
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"math"
	"time"
)

// Facts must be independently selected, never taken from the inspected document.
// Duty validity and Now are offline assertions, not verified Time Confidence.
type Facts struct {
	Network, Issuer, Authority, Holder [32]byte
	Duty                               uint64
	DutyNotBefore, DutyNotAfter, Now   time.Time
	Class                              uint8
	Count                              uint32
}

// Outcome is a finite diagnostic category containing no inspected data.
type Outcome string

const (
	Accepted     Outcome = "accepted-offline"
	InvalidInput Outcome = "invalid-input"
	Malformed    Outcome = "malformed-permission"
	Signature    Outcome = "invalid-signature"
	Binding      Outcome = "binding-mismatch"
	Validity     Outcome = "outside-validity"
	Limit        Outcome = "request-limit"
	Canceled     Outcome = "canceled"
)

// Inspect checks a fixed 228-byte permission. Accepted does not authenticate a
// live holder or account for preceding requests. All inputs remain caller-owned.
func Inspect(ctx context.Context, raw []byte, facts Facts) Outcome {
	return inspectPermission(ctx, raw, facts)
}

func inspectPermission(ctx context.Context, raw []byte, facts Facts) Outcome {
	if ctx == nil {
		return InvalidInput
	}
	if ctx.Err() != nil {
		return Canceled
	}
	zero := [32]byte{}
	if facts.Network == zero || facts.Issuer == zero || facts.Authority == zero || facts.Holder == zero || facts.Duty == 0 || facts.Class < 1 || facts.Class > 3 || facts.Count == 0 || facts.Count > 65536 || facts.DutyNotBefore.IsZero() || !facts.DutyNotAfter.After(facts.DutyNotBefore) || facts.Now.IsZero() {
		return InvalidInput
	}
	if len(raw) != 228 {
		return Malformed
	}
	var network, issuer, permission, holder [32]byte
	copy(network[:], raw[0:32])
	copy(issuer[:], raw[32:64])
	duty := binary.BigEndian.Uint64(raw[64:72])
	copy(permission[:], raw[72:104])
	copy(holder[:], raw[104:136])
	start := binary.BigEndian.Uint64(raw[136:144])
	end := binary.BigEndian.Uint64(raw[144:152])
	var maxima [3]uint32
	var any bool
	for i := range maxima {
		maxima[i] = binary.BigEndian.Uint32(raw[152+i*4 : 156+i*4])
		if maxima[i] > 65536 {
			return Malformed
		}
		any = any || maxima[i] != 0
	}
	if network == zero || issuer == zero || permission == zero || holder == zero || duty == 0 || start > math.MaxInt64-3600 || start%3600 != 0 || end != start+3600 || !any {
		return Malformed
	}
	transcript := append([]byte("ardents-issuance-permission-v1\x00"), raw[:164]...)
	if !ed25519.Verify(ed25519.PublicKey(facts.Authority[:]), transcript, raw[164:]) {
		return Signature
	}
	if ctx.Err() != nil {
		return Canceled
	}
	if network != facts.Network || issuer != facts.Issuer || duty != facts.Duty || holder != facts.Holder {
		return Binding
	}
	before, after := time.Unix(int64(start), 0), time.Unix(int64(end), 0)
	if before.Before(facts.DutyNotBefore) || after.After(facts.DutyNotAfter) || facts.Now.Before(before) || !facts.Now.Before(after) {
		return Validity
	}
	if facts.Count > maxima[facts.Class-1] {
		return Limit
	}
	return Accepted
}
