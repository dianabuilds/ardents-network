package endpoint

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// SuccessorRecovery retains one exact successor intent and independent native
// custody. A complete staged prefix can continue through fresh proof admission,
// original predecessor join and guarded start. Terminal cleanup retains the
// existing exact invocation without replaying ACK, Stop or Start. Other prefixes
// refuse with ErrRepairRequired.
type SuccessorRecovery struct{ state *successorRecoveryOperation }

type successorRecoveryOperation struct {
	mu        sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
	native    *successorRecoveryNative
	request   installationRequest
	reference time.Time
	used      bool
	result    ProvisionResult
	terminal  error
}

// OpenSuccessorRecovery obtains Installation custody before composition reads
// Candidate bytes or opens the separate retained Release history. Its original
// bound includes acquisition; copies cannot create another admission lifetime.
func OpenSuccessorRecovery(ctx context.Context, root string, reference time.Time) (*SuccessorRecovery, error) {
	if ctx == nil || !canonicalPath(root) || root == "/" || reference.IsZero() {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
	native, request, err := openSuccessorRecovery(bounded, root, reference.UTC())
	if err != nil {
		cancel()
		return nil, err
	}
	return &SuccessorRecovery{state: &successorRecoveryOperation{ctx: bounded, cancel: cancel, native: native, request: request, reference: reference.UTC()}}, nil
}

func (r *SuccessorRecovery) BundleRoot() string {
	if r == nil || r.state == nil {
		return ""
	}
	return r.state.request.BundleRoot
}
func (r *SuccessorRecovery) ReleaseHistoryRoot() string {
	if r == nil || r.state == nil {
		return ""
	}
	return r.state.request.ReleaseFloorRoot
}
func (r *SuccessorRecovery) ReferenceTime() time.Time {
	if r == nil || r.state == nil {
		return time.Time{}
	}
	return r.state.reference
}

// Complete obtains two fresh private proofs through the retained verifier under
// its ORIGINAL opening before any cleanup. A caller cannot replay an earlier
// Authorization. The fixed unit and original process are reobserved at handoff.
func (r *SuccessorRecovery) Complete(verifier *release.Verifier, candidate enrollment.Candidate, input release.Inputs) (ProvisionResult, error) {
	if r == nil || r.state == nil || r.state.ctx == nil {
		return ProvisionResult{}, ErrInput
	}
	s := r.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used || s.native == nil {
		return s.result, errors.Join(ErrInput, s.terminal)
	}
	s.used = true
	var result ProvisionResult
	var err error
	if err = s.ctx.Err(); err == nil {
		result, err = completeSuccessorRecovery(s.ctx, s.native, s.reference, verifier, candidate, input)
	}
	s.terminal = errors.Join(err, closeSuccessorRecovery(s.native), s.ctx.Err())
	s.result = result
	s.native = nil
	s.cancel()
	if s.terminal != nil {
		if s.result.Status != "" {
			s.result.Status = "installed-started-recovery-required"
		}
		return s.result, s.terminal
	}
	return result, nil
}

// Close joins any originally admitted physical work before releasing the writer.
// It never revokes a fully acknowledged invocation or performs another recovery.
func (r *SuccessorRecovery) Close() error {
	if r == nil || r.state == nil {
		return nil
	}
	s := r.state
	s.mu.Lock()
	defer s.mu.Unlock()
	s.used = true
	if s.native != nil {
		s.terminal = errors.Join(s.terminal, closeSuccessorRecovery(s.native), s.ctx.Err())
		s.native = nil
		s.cancel()
	}
	return s.terminal
}
