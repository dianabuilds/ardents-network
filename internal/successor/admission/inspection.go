package admission

import (
	"context"
	"crypto/ed25519"

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
	permission, err := decodePermission(raw, false)
	if err != nil {
		return Malformed
	}
	if !ed25519.Verify(ed25519.PublicKey(facts.Authority[:]), PermissionTranscript(permission), permission.Signature[:]) {
		return Signature
	}
	if ctx.Err() != nil {
		return Canceled
	}
	if permission.NetworkID != facts.Network || permission.IssuerNodeID != facts.Issuer || permission.DutyGeneration != facts.Duty || permission.HolderKey != facts.Holder {
		return Binding
	}
	before, after := permission.NotBefore, permission.NotAfter
	if before.Before(facts.DutyNotBefore) || after.After(facts.DutyNotAfter) || facts.Now.Before(before) || !facts.Now.Before(after) {
		return Validity
	}
	if facts.Count > permission.Maxima[facts.Class-1] {
		return Limit
	}
	return Accepted
}
