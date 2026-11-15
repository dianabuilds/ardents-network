package infrastructure

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// ForwardingAuthorization privately retains the exact Forwarding artifact pair.
// It cannot be passed as Source Authorization or reconstructed from public facts.
// Native installation and independent startup still require their own admission.
type ForwardingAuthorization struct{ pair Authorization }

func (a ForwardingAuthorization) Check(ctx context.Context, v *release.Verifier) error {
	return a.pair.Check(ctx, v)
}
func (a ForwardingAuthorization) Targets() (release.Authorization, release.Authorization) {
	return a.pair.Targets()
}
func (a ForwardingAuthorization) Descriptor() []byte { return a.pair.Descriptor() }
func (a ForwardingAuthorization) Program() []byte    { return a.pair.Program() }
func (a ForwardingAuthorization) Template() []byte   { return a.pair.Template() }
func (a ForwardingAuthorization) InitialFacts() (enrollment.InfrastructureFacts, bool) {
	return a.pair.InitialFacts()
}

// Artifacts returns only detached bytes from the exact frozen Forwarding
// inventory. It grants no proof, initial provenance, mutation or startup right.
func (a ForwardingAuthorization) Artifacts() map[string][]byte {
	if a.pair.origin == nil {
		return nil
	}
	files := make(map[string][]byte, 8)
	for _, name := range []string{ProgramName, ForwardingTemplateName, ForwardingDescriptorName, "infrastructure-enrollment.json", "root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		raw, ok := a.pair.origin.File(name)
		if !ok {
			return nil
		}
		files[name] = raw
	}
	return files
}

func AuthenticateForwardingInitial(ctx context.Context, v *release.Verifier, b enrollment.InfrastructureBundle, input release.Inputs) (ForwardingAuthorization, error) {
	f, ok := b.Facts()
	if !ok || f.Invocation != "forwarding" || ctx == nil || v == nil {
		return ForwardingAuthorization{}, ErrInput
	}
	floors, err := v.CurrentFloors(ctx)
	if err != nil {
		return ForwardingAuthorization{}, err
	}
	if !emptyInitialFloors(floors) {
		return ForwardingAuthorization{}, release.ErrTrustUnavailable
	}
	a, err := authenticateInventory(ctx, v, b, f, input, artifactProfile{"forwarding", ForwardingTemplateName, ForwardingDescriptorName, ForwardingGenerationTarget})
	if err != nil {
		return ForwardingAuthorization{}, err
	}
	p, valid := a.program.AcceptedDecision()
	if !valid || p.Outcome != release.OutcomeReleaseAccepted {
		return ForwardingAuthorization{}, release.ErrTrustUnavailable
	}
	a.initial = b
	return ForwardingAuthorization{pair: a}, nil
}

func AuthenticateForwardingCandidate(ctx context.Context, v *release.Verifier, c enrollment.InfrastructureCandidate, input release.Inputs) (ForwardingAuthorization, error) {
	f, ok := c.Facts()
	if !ok || f.Invocation != "forwarding" || ctx == nil || v == nil {
		return ForwardingAuthorization{}, ErrInput
	}
	floors, err := v.CurrentFloors(ctx)
	if err != nil {
		return ForwardingAuthorization{}, err
	}
	if !floors.Complete() {
		return ForwardingAuthorization{}, release.ErrTrustUnavailable
	}
	a, err := authenticateInventory(ctx, v, c, f, input, artifactProfile{"forwarding", ForwardingTemplateName, ForwardingDescriptorName, ForwardingGenerationTarget})
	if err != nil {
		return ForwardingAuthorization{}, err
	}
	return ForwardingAuthorization{pair: a}, nil
}
