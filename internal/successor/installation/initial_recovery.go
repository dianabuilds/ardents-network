package installation

import (
	"bytes"
	"context"
	"errors"
	"path"
	"sync"
	"time"
)

var ErrRepairRequired = errors.New("installation: explicit repair required")

type initialTransitionIntent struct {
	Schema           string              `json:"schema"`
	Previous         generationSelection `json:"previous"`
	Candidate        generationSelection `json:"candidate"`
	CandidateBinding generationBinding   `json:"candidate_binding"`
	Request          installationRequest `json:"request"`
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
	request   installationRequest
	reference time.Time
	used      bool
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
func (r *Recovery) Complete(authorization Authorization) (ProvisionResult, error) {
	if r == nil || r.state == nil || r.state.ctx == nil {
		return ProvisionResult{}, ErrInput
	}
	state := r.state
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.used || state.native == nil {
		return ProvisionResult{}, errors.Join(ErrInput, state.terminal)
	}
	state.used = true
	ctx := state.ctx
	if err := ctx.Err(); err != nil {
		state.terminal = errors.Join(err, closeInitialRecovery(state.native))
		state.native = nil
		return ProvisionResult{}, state.terminal
	}
	result, err := completeInitialRecovery(ctx, state, authorization)
	state.terminal = errors.Join(err, closeInitialRecovery(state.native), ctx.Err())
	state.native = nil
	if state.terminal != nil {
		return ProvisionResult{}, state.terminal
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
	defer state.mu.Unlock()
	state.used = true
	if state.native != nil {
		state.terminal = errors.Join(state.terminal, closeInitialRecovery(state.native))
		state.native = nil
	}
	if state.ctx != nil {
		state.terminal = errors.Join(state.terminal, state.ctx.Err())
	}
	return state.terminal
}

func recoveryGeneration(ctx context.Context, intent initialTransitionIntent, reference time.Time, authorization Authorization) (inspectedGeneration, error) {
	if err := ctx.Err(); err != nil {
		return inspectedGeneration{}, err
	}
	p, programOK := authorization.program.AcceptedDecision()
	g, generationOK := authorization.generation.AcceptedDecision()
	if !programOK || !generationOK || !p.ReferenceTime.Equal(reference) || !g.ReferenceTime.Equal(reference) {
		return inspectedGeneration{}, ErrAuthorization
	}
	for _, pair := range []struct{ old, fresh targetObservation }{
		{intent.CandidateBinding.Program, observeTarget(p)}, {intent.CandidateBinding.Generation, observeTarget(g)},
	} {
		old, fresh := pair.old, pair.fresh
		before, err := time.Parse(time.RFC3339Nano, old.ReferenceTime)
		if err != nil || reference.Before(before) || old.Path != fresh.Path || old.Digest != fresh.Digest || old.Length != fresh.Length ||
			old.ReleaseIdentity != fresh.ReleaseIdentity || old.ReleaseVersion != fresh.ReleaseVersion || old.Platform != fresh.Platform || old.Architecture != fresh.Architecture ||
			old.Environment != fresh.Environment || old.Network != fresh.Network || fresh.TargetsVersion < old.TargetsVersion ||
			(fresh.TargetsVersion == old.TargetsVersion && fresh.TargetsDigest != old.TargetsDigest) {
			return inspectedGeneration{}, ErrBinding
		}
	}
	files := make(map[string][]byte, 14)
	for name, body := range authorization.resources {
		files[name] = bytes.Clone(body)
	}
	files["protected-endpoint.json"] = bytes.Clone(authorization.descriptor)
	request := intent.Request
	var err error
	files["request.json"], err = canonicalJSON(request)
	if err != nil {
		return inspectedGeneration{}, err
	}
	plan := request.Headless
	plan.NetworkSourcePlan = path.Join(request.InstallationRoot, "generations", intent.Candidate.GenerationDigest, "source.json")
	files["headless.json"], err = canonicalJSON(plan)
	if err != nil {
		return inspectedGeneration{}, err
	}
	files["source.json"], err = canonicalJSON(request.Source)
	if err != nil {
		return inspectedGeneration{}, err
	}
	files["endpoint-unit.service"], err = renderEndpointUnit(files["ardents-endpoint.service"], request, path.Join(request.InstallationRoot, "generations", intent.Candidate.GenerationDigest))
	if err != nil {
		return inspectedGeneration{}, err
	}
	selected, selectionErr := canonicalJSON(intent.Candidate)
	binding, bindingErr := canonicalJSON(intent.CandidateBinding)
	if err := errors.Join(selectionErr, bindingErr, ctx.Err()); err != nil {
		return inspectedGeneration{}, err
	}
	return inspectGeneration(request.InstallationRoot, selected, binding, files)
}
