package issuer

import (
	"context"
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
	o, opened := OpenCurrent(ctx, p, observe)
	if opened.Outcome != "opened" {
		return opened
	}
	r := o.IssueCurrent(ctx, raw, kind)
	closed := o.Close(context.Background())
	r.Cleanup = closed.Cleanup
	if o.Err() != nil {
		if closed.Outcome != "closed" && r.Response != nil {
			r.Phase = "authority"
			r.Outcome = "authority-unavailable"
		}
		clear(r.Response)
		r.Response = nil
	}
	for _, completion := range r.Cleanup {
		if completion.Phase != "" && completion.Outcome != "closed" {
			clear(r.Response)
			r.Response = nil
		}
	}
	if r.Response != nil {
		current, at, err := observe()
		if err != nil || current != o.state.initial || current.ValidateAt(at) != nil || at.Before(o.state.lastIssueAt) || at.Before(request.Permission.NotBefore) || !at.Before(request.Permission.NotAfter) || ctx.Err() != nil {
			clear(r.Response)
			r.Response = nil
			r.Phase = "authority"
			r.Outcome = "authority-unavailable"
		}
	}
	return r
}

func matchesAuthority(b quota.LedgerBinding, current admission.AuthorityFacts) bool {
	if current.NetworkID != b.Network || current.IssuerNodeID != b.Issuer || current.IssuanceAuthorityKey != b.Authority || current.Digest != b.Profile ||
		current.IssuerDutyGeneration != b.Duty || !current.NotBefore.Equal(b.Start) || !current.NotAfter.Equal(b.End) || int(current.TokenKeyCount) != len(b.Keys) {
		return false
	}
	for i, k := range b.Keys {
		v := current.TokenKeys[i]
		if v.Class != k.Class || uint64(v.WindowStart.Unix()) != k.Window || string(v.SPKI[:]) != string(k.SPKI) {
			return false
		}
	}
	return true
}
