package delivery

import (
	"context"
	"errors"
	"sync"
	"time"

	requestinput "github.com/dianabuilds/ardents-network/internal/successor/endpoint/delivery/request"
	generationauthorization "github.com/dianabuilds/ardents-network/internal/successor/release/endpoint"
)

var ErrRepairRequired = errors.New("installation: explicit repair required")

type initialTransitionIntent struct {
	Schema           string                   `json:"schema"`
	Previous         generationSelection      `json:"previous"`
	Candidate        generationSelection      `json:"candidate"`
	CandidateBinding generationBinding        `json:"candidate_binding"`
	Request          requestinput.Declaration `json:"request"`
}

// Recovery retains one original intent and its Installation lease. Composition
// obtains fresh Candidate/Release proofs while this owner remains open. It is
// neither a new initial pin nor permission to select a different generation.
type Recovery struct {
	state *recoveryOperation
}

// Copies of the opaque handle share the same admission latch and physical
// lifetime. A struct copy cannot create another completion or writer owner.
type recoveryOperation struct {
	mu        sync.Mutex
	ctx       context.Context
	native    *recoveryNative
	request   requestinput.Declaration
	reference time.Time
	used      bool
	// busy serializes Complete and Close while native I/O runs without mu.
	busy      chan struct{}
	terminal  error
}

// OpenInitialRecovery observes native custody before Release history effects.
// An incomplete birth inventory requires repair; it is never adopted or reset.
func OpenInitialRecovery(ctx context.Context, root string, reference time.Time) (*Recovery, error) {
	if ctx == nil || !canonicalPath(root) || root == "/" || reference.IsZero() {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return openInitialRecovery(ctx, root, reference.UTC())
}

func (r *Recovery) BundleRoot() string {
	if r == nil || r.state == nil {
		return ""
	}
	return r.state.request.BundleRoot
}
func (r *Recovery) ReleaseHistoryRoot() string {
	if r == nil || r.state == nil {
		return ""
	}
	return r.state.request.ReleaseFloorRoot
}
func (r *Recovery) ReferenceTime() time.Time {
	if r == nil || r.state == nil {
		return time.Time{}
	}
	return r.state.reference
}

// Complete uses two genuine fresh authorizations and only the original bytes.
// It retains OpenInitialRecovery's original caller and leaves provisioning
// stopped. There is no replacement context that can renew an expired opening.
func (r *Recovery) Complete(authorization generationauthorization.Authorization) (ProvisionResult, error) {
	if r == nil || r.state == nil || r.state.ctx == nil {
		return ProvisionResult{}, ErrInput
	}
	state := r.state
	state.mu.Lock()
	for state.busy != nil {
		busy := state.busy
		state.mu.Unlock()
		<-busy
		state.mu.Lock()
	}
	if state.used || state.native == nil {
		err := errors.Join(ErrInput, state.terminal)
		state.mu.Unlock()
		return ProvisionResult{}, err
	}
	state.used = true
	state.busy = make(chan struct{})
	ctx := state.ctx
	state.mu.Unlock()
	var result ProvisionResult
	err := ctx.Err()
	if err == nil {
		result, err = completeInitialRecovery(ctx, state, authorization)
	}
	state.terminal = errors.Join(err, closeInitialRecovery(state.native), ctx.Err())
	state.native = nil
	state.mu.Lock()
	close(state.busy)
	state.busy = nil
	err = state.terminal
	state.mu.Unlock()
	if err != nil {
		return ProvisionResult{}, err
	}
	return result, nil
}

// Close joins owned filesystem borrowers before releasing the writer. Closing
// without Complete has no installation effect and cannot erase Release floors.
func (r *Recovery) Close() error {
	if r == nil || r.state == nil {
		return nil
	}
	state := r.state
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
	if state.native != nil {
		state.terminal = errors.Join(state.terminal, closeInitialRecovery(state.native))
		state.native = nil
	}
	if state.ctx != nil {
		state.terminal = errors.Join(state.terminal, state.ctx.Err())
	}
	state.mu.Lock()
	close(state.busy)
	state.busy = nil
	err := state.terminal
	state.mu.Unlock()
	return err
}

func recoveryGeneration(ctx context.Context, intent initialTransitionIntent, reference time.Time, authorization generationauthorization.Authorization) (inspectedGeneration, error) {
	return recoverBoundGeneration(ctx, intent.Candidate, intent.CandidateBinding, intent.Request, reference, authorization)
}
