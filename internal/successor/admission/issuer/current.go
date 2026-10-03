package issuer

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"time"
)

// IssueCurrent uses the same quota/key/result transaction as offline Issue.
// The observer supplies current external authority; Admission never authenticates
// Network membership itself. Every observation must match the original binding.
func IssueCurrent(ctx context.Context, p Plan, raw []byte, kind quota.Kind, observe func() (admission.AuthorityFacts, time.Time, error)) Result {
	if ctx == nil || observe == nil {
		return Result{Phase: "authority", Outcome: "invalid-input"}
	}
	request, err := admission.DecodeClosedTokenBatch(raw)
	if err != nil {
		return Result{Phase: "input", Outcome: "invalid-input"}
	}
	initial, now, err := observe()
	if err != nil || initial.ValidateAt(now) != nil {
		return Result{Phase: "authority", Outcome: "authority-unavailable"}
	}
	b := p.AdmissionBinding
	if initial.NetworkID != b.Network || initial.IssuerNodeID != b.Issuer || initial.IssuanceAuthorityKey != b.Authority || initial.Digest != b.Profile ||
		initial.IssuerDutyGeneration != b.Duty || !initial.NotBefore.Equal(b.Start) || !initial.NotAfter.Equal(b.End) || int(initial.TokenKeyCount) != len(b.Keys) {
		return Result{Phase: "authority", Outcome: "binding-mismatch"}
	}
	for i, k := range b.Keys {
		v := initial.TokenKeys[i]
		if v.Class != k.Class || uint64(v.WindowStart.Unix()) != k.Window || string(v.SPKI[:]) != string(k.SPKI) {
			return Result{Phase: "authority", Outcome: "binding-mismatch"}
		}
	}
	f := admission.Facts{Network: b.Network, Issuer: b.Issuer, Authority: b.Authority, Holder: request.Permission.HolderKey, Duty: b.Duty, DutyNotBefore: b.Start, DutyNotAfter: b.End, Now: now.UTC().Truncate(time.Second), Class: request.Class, Count: uint32(len(request.BlindedRequests))}
	floor := now
	recheck := func() (time.Time, error) {
		if ctx.Err() != nil {
			return time.Time{}, ctx.Err()
		}
		current, at, err := observe()
		if err != nil || current != initial || at.Before(floor) || current.ValidateAt(at) != nil || at.Before(request.Permission.NotBefore) || !at.Before(request.Permission.NotAfter) {
			return time.Time{}, errors.Join(errors.New("issuance authority changed"), err)
		}
		floor = at
		return at.UTC().Truncate(time.Second), nil
	}
	return execute(ctx, p, raw, f, kind, false, recheck)
}
