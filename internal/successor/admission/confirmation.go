package admission

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"time"
)

// DebitConfirmation cannot be constructed outside Admission. It confirms only
// durable offline accounting, never authenticated State or a network lane.
type DebitConfirmation struct{ state *confirmedDebit }
type confirmedDebit struct {
	raw     []byte
	binding LedgerBinding
	kind    Kind
	checked time.Time
}

func cloneBinding(b LedgerBinding) LedgerBinding {
	keys := make([]TokenKey, len(b.Keys))
	for i, k := range b.Keys {
		keys[i] = k
		keys[i].SPKI = append([]byte(nil), k.SPKI...)
	}
	b.Keys = keys
	return b
}

// Snapshot exposes defensive verified data without providing a mint operation.
func (c DebitConfirmation) Snapshot() ([]byte, LedgerBinding, Kind, time.Time, bool) {
	if c.state == nil {
		return nil, LedgerBinding{}, 0, time.Time{}, false
	}
	s := c.state
	return append([]byte(nil), s.raw...), cloneBinding(s.binding), s.kind, s.checked, true
}

// BindingDigest identifies the exact canonical persisted offline binding.
func BindingDigest(b LedgerBinding) ([32]byte, error) {
	if !b.valid() {
		return [32]byte{}, ErrInvalid
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return [32]byte{}, ErrInvalid
	}
	return sha256.Sum256(raw), nil
}

// ValidateBatch checks retained request evidence without issuing confirmation.
// It is suitable for journal integrity checks; it never debits or authorizes.
func ValidateBatch(ctx context.Context, raw []byte, b LedgerBinding, now time.Time) Outcome {
	if ctx == nil || !b.valid() || len(raw) < 625 {
		return InvalidInput
	}
	f := Facts{Network: b.Network, Issuer: b.Issuer, Authority: b.Authority, Duty: b.Duty, DutyNotBefore: b.Start, DutyNotAfter: b.End, Now: now, Class: raw[268], Count: uint32(raw[623])<<8 | uint32(raw[624])}
	copy(f.Holder[:], raw[112:144])
	_, outcome := verifyBatch(ctx, raw, f, b)
	return outcome
}

// DebitVerified mints confirmation only after the shared transaction commits.
func (l *Ledger) DebitVerified(ctx context.Context, raw []byte, f Facts, kind Kind) (Outcome, DebitConfirmation) {
	var confirmation DebitConfirmation
	if len(raw) > 16<<10 {
		return Malformed, confirmation
	}
	// Copy before validation: callers own their buffers; returned evidence cannot
	// alias them. Concurrent mutation of an input during this call is unsupported.
	owned := append([]byte(nil), raw...)
	outcome := l.debit(ctx, owned, f, kind, func(verifiedBatch) {
		confirmation.state = &confirmedDebit{raw: owned, binding: cloneBinding(l.state.binding), kind: kind, checked: f.Now}
	})
	if confirmation.state == nil {
		clear(owned)
	}
	return outcome, confirmation
}
