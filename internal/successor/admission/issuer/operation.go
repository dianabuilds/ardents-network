package issuer

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
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

// Completion records one phase without retaining raw errors or private inputs.
type Completion struct {
	Phase, Outcome string
}

// Result retains the primary operation and each acquired owner's cleanup.
// Cleanup slots follow results/profile, keys, admission/identity retirement order.
// Status projects the existing command outcome; any cleanup failure suppresses
// response export without erasing the operation or another owner's result.
type Result struct {
	Phase, Outcome string
	Response       []byte
	Cleanup        [3]Completion
}

func (r Result) Status() (phase, outcome string) {
	phase, outcome = r.Phase, r.Outcome
	for _, closed := range r.Cleanup {
		if closed.Phase != "" && closed.Outcome != "closed" {
			phase, outcome = closed.Phase, "storage-uncertain"
		}
	}
	return phase, outcome
}

func (r *Result) closeOwner(slot int, phase string, close func() error) {
	result := Completion{Phase: phase, Outcome: "closed"}
	if err := close(); err != nil {
		result.Outcome = category(err)
		r.Response = nil
	}
	r.Cleanup[slot] = result
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
	defer r.closeOwner(2, "close-admission", ledger.Close)
	r.Phase = "open-keys"
	store, err := issuance.Open(ctx, p.KeyRoot, p.KeyBinding)
	if err != nil {
		r.Outcome = category(err)
		return r
	}
	defer r.closeOwner(1, "close-keys", store.Close)
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
	defer r.closeOwner(0, "close-results", results.Close)
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
