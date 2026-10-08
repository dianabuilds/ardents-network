package installation

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Successor owns one explicit replacement attempt. Copies share its original
// caller, writer lease and one-use admission latch. Composition keeps the
// separate Release verifier open until Close has joined Installation borrowers.
type Successor struct{ state *successorOperation }

type successorOperation struct {
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	request  Request
	native   *successorPreparation
	used     bool
	result   ProvisionResult
	terminal error
}

// OpenSuccessor checks native request and installed custody before composition
// loads an unpinned Candidate or opens retained Release history. The two-minute
// replacement bound includes proof acquisition; cleanup still owns unfinished
// original work after expiry. Opening grants no replacement or startup authority.
func OpenSuccessor(ctx context.Context, filename string) (*Successor, error) {
	if ctx == nil || filename == "" {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
	native, request, err := openSuccessor(bounded, filename)
	if err != nil {
		cancel()
		return nil, err
	}
	return &Successor{state: &successorOperation{ctx: bounded, cancel: cancel, request: request, native: native}}, nil
}

// Request exposes immutable acquisition inputs, not installed authority.
func (s *Successor) Request() Request {
	if s == nil || s.state == nil {
		return Request{}
	}
	return s.state.request
}

// Complete authenticates both targets against the original installed binding,
// stages, joins the predecessor, replaces and selects, then attempts fixed-unit
// startup under its barrier. There is no replacement caller or implicit retry.
// A fully written ACK is irreversible: subsequent failure retains
// installed-started-recovery-required with an error instead of an empty result.
func (s *Successor) Complete(verifier *release.Verifier, candidate enrollment.Candidate, input release.Inputs) (ProvisionResult, error) {
	if s == nil || s.state == nil {
		return ProvisionResult{}, ErrInput
	}
	state := s.state
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.used || state.native == nil {
		return state.result, errors.Join(ErrInput, state.terminal)
	}
	state.used = true
	state.result, state.terminal = completeSuccessor(state.native, verifier, candidate, input)
	state.finishClose()
	return state.result, state.terminal
}

// Close retains original failed physical custody until join. It cannot stop a
// fully acknowledged invocation, retry completion or clear the first outcome.
func (s *Successor) Close() error {
	if s == nil || s.state == nil {
		return nil
	}
	state := s.state
	state.mu.Lock()
	defer state.mu.Unlock()
	state.used = true
	state.finishClose()
	return state.terminal
}

func (state *successorOperation) finishClose() {
	if state.native != nil {
		err, released := closeSuccessor(state.native)
		state.terminal = errors.Join(state.terminal, err)
		if released {
			state.native = nil
		}
	}
	if state.ctx != nil {
		state.terminal = errors.Join(state.terminal, state.ctx.Err())
	}
	if state.result.Status != "" && state.terminal != nil {
		state.result.Status = "installed-started-recovery-required"
	}
	if state.native == nil && state.cancel != nil {
		state.cancel()
		state.cancel = nil
		// Internal disposal must not introduce a new caller-cancellation outcome.
		state.ctx = nil
	}
}
