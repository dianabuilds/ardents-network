package tokenissuance

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/issuance"
	"github.com/dianabuilds/ardents-network/internal/successor/nodeidentity"
)

// Plan fixes three independently owned non-nested roots and offline bindings.
type Plan struct {
	AdmissionRoot    string
	AdmissionBinding admission.LedgerBinding
	KeyRoot          string
	KeyBinding       issuance.Binding
	ResultRoot       string
}

// Result exposes finite lifecycle categories and successful response bytes only.
type Result struct {
	Phase, Outcome string
	Response       []byte
}

func category(err error) string {
	switch {
	case errors.Is(err, issuance.ErrUncertain), errors.Is(err, admission.ErrUncertain), errors.Is(err, nodeidentity.ErrUncertain):
		return "storage-uncertain"
	case errors.Is(err, issuance.ErrUnsupported), errors.Is(err, admission.ErrUnsupported), errors.Is(err, nodeidentity.ErrUnsupported):
		return "unsupported-platform"
	case errors.Is(err, issuance.ErrBusy), errors.Is(err, admission.ErrBusy), errors.Is(err, nodeidentity.ErrBusy):
		return "busy"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	case errors.Is(err, issuance.ErrInvalid), errors.Is(err, admission.ErrInvalid), errors.Is(err, nodeidentity.ErrInvalid):
		return "invalid-input"
	case errors.Is(err, issuance.ErrValidity):
		return "outside-validity"
	case errors.Is(err, issuance.ErrConflict):
		return "request-conflict"
	case errors.Is(err, issuance.ErrCapacity):
		return "result-capacity"
	default:
		return "storage-unavailable"
	}
}
func (p Plan) valid() bool {
	roots := []string{p.AdmissionRoot, p.KeyRoot, p.ResultRoot}
	for i, a := range roots {
		if !filepath.IsAbs(a) || filepath.Clean(a) != a {
			return false
		}
		for j, b := range roots {
			if i != j && (a == b || strings.HasPrefix(a, b+string(filepath.Separator))) {
				return false
			}
		}
	}
	return true
}

// Initialize creates only a fresh result root against existing ledger and keys.
func Initialize(ctx context.Context, p Plan) Result {
	return execute(ctx, p, nil, admission.Facts{}, 0, true)
}

// Issue opens all owners before any debit and closes them in reverse order.
func Issue(ctx context.Context, p Plan, raw []byte, f admission.Facts, kind admission.Kind) Result {
	return execute(ctx, p, raw, f, kind, false)
}
func execute(ctx context.Context, p Plan, raw []byte, f admission.Facts, kind admission.Kind, initialize bool) (r Result) {
	r = Result{Phase: "input", Outcome: "invalid-input"}
	if !issuance.Supported() {
		r.Outcome = "unsupported-platform"
		return r
	}
	if ctx == nil || !p.valid() {
		return r
	}
	if ctx.Err() != nil {
		r.Outcome = "canceled"
		return r
	}
	r.Phase = "open-admission"
	ledger, err := admission.Open(p.AdmissionRoot, p.AdmissionBinding)
	if err != nil {
		r.Outcome = category(err)
		return r
	}
	defer func() {
		if e := ledger.Close(); e != nil {
			r.Phase = "close-admission"
			r.Outcome = "storage-uncertain"
			r.Response = nil
		}
	}()
	r.Phase = "open-keys"
	store, err := issuance.Open(ctx, p.KeyRoot, p.KeyBinding)
	if err != nil {
		r.Outcome = category(err)
		return r
	}
	defer func() {
		if e := store.Close(); e != nil {
			r.Phase = "close-keys"
			r.Outcome = "storage-uncertain"
			r.Response = nil
		}
	}()
	r.Phase = "open-results"
	if initialize {
		err = issuance.InitializeResults(ctx, p.ResultRoot, store, p.AdmissionBinding)
		r.Outcome = "initialized-results"
		if err != nil {
			r.Outcome = category(err)
		}
		return r
	}
	results, err := issuance.OpenResults(ctx, p.ResultRoot, store, p.AdmissionBinding)
	if err != nil {
		r.Outcome = category(err)
		return r
	}
	defer func() {
		if e := results.Close(); e != nil {
			r.Phase = "close-results"
			r.Outcome = "storage-uncertain"
			r.Response = nil
		}
	}()
	r.Phase = "debit"
	outcome, confirmation := ledger.DebitVerified(ctx, raw, f, kind)
	if outcome != admission.Debited && outcome != admission.AlreadyDebited {
		r.Outcome = string(outcome)
		return r
	}
	r.Phase = "issue"
	response, replay, err := results.Issue(ctx, confirmation, f.Now)
	if err != nil {
		r.Outcome = category(err)
		return r
	}
	r.Outcome = "issued-offline"
	if replay {
		r.Outcome = "already-issued"
	}
	r.Response = response
	return r
}
