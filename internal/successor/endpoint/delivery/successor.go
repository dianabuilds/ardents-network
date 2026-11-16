package delivery

import (
	"context"
	"errors"
	"sync"
	"time"

	requestinput "github.com/dianabuilds/ardents-network/internal/successor/endpoint/delivery/request"
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
	request  requestinput.Document
	native   *successorPreparation
	used     bool
	// busy serializes Complete and Close while native I/O runs without mu.
	busy     chan struct{}
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
func (s *Successor) Request() requestinput.Document {
	if s == nil || s.state == nil {
		return requestinput.Document{}
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
	for state.busy != nil {
		busy := state.busy
		state.mu.Unlock()
		<-busy
		state.mu.Lock()
	}
	if state.used || state.native == nil {
		result, err := state.result, errors.Join(ErrInput, state.terminal)
		state.mu.Unlock()
		return result, err
	}
	state.used = true
	state.busy = make(chan struct{})
	state.mu.Unlock()
	result, err := completeSuccessor(state.native, verifier, candidate, input)
	state.terminal = err
	state.result = result
	state.finishClose()
	state.mu.Lock()
	close(state.busy)
	state.busy = nil
	result, err = state.result, state.terminal
	state.mu.Unlock()
	return result, err
}

// Close retains original failed physical custody until join. It cannot stop a
// fully acknowledged invocation, retry completion or clear the first outcome.
func (s *Successor) Close() error {
	if s == nil || s.state == nil {
		return nil
	}
	state := s.state
	state.mu.Lock()
	for state.busy != nil {
		busy := state.busy
		state.mu.Unlock()
		<-busy
		state.mu.Lock()
	}
	state.used = true
	state.busy = make(chan struct{})
	state.mu.Unlock()
	state.finishClose()
	state.mu.Lock()
	close(state.busy)
	state.busy = nil
	err := state.terminal
	state.mu.Unlock()
	return err
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
