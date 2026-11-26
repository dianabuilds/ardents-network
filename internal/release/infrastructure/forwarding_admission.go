//go:build linux

package infrastructure

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/domain/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
)

// ForwardingAuthorization privately retains the exact Forwarding artifact pair.
// It cannot be passed as Source Authorization or reconstructed from public facts.
// The platform-led lifecycle owns installed admission; this candidate pair
// serves the distinct forward-Release predecessor verification only. The former
// initial admission entry point and detached inventory accessors retired with
// the transitional Forwarding orchestration (#604 FJD stage D, ADR-0125).
type ForwardingAuthorization struct{ pair Authorization }

func (a ForwardingAuthorization) Check(ctx context.Context, v *release.Verifier) error {
	return a.pair.Check(ctx, v)
}
func (a ForwardingAuthorization) Targets() (release.Authorization, release.Authorization) {
	return a.pair.Targets()
}
func (a ForwardingAuthorization) InitialFacts() (enrollment.InfrastructureFacts, bool) {
	return a.pair.InitialFacts()
}

func AuthenticateForwardingCandidate(ctx context.Context, v *release.Verifier, c enrollment.InfrastructureCandidate, input release.Inputs) (ForwardingAuthorization, error) {
	f, ok := c.Facts()
	if !ok || f.Invocation != "forwarding" || ctx == nil || v == nil || f.Platform != "linux-amd64" || input.TargetPath != ProgramTarget || input.Local.Platform != f.Platform || input.Local.Architecture != "amd64" || input.Local.RefTime.IsZero() {
		return ForwardingAuthorization{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return ForwardingAuthorization{}, err
	}
	floors, err := v.CurrentFloors(ctx)
	if err != nil {
		return ForwardingAuthorization{}, err
	}
	if !floors.Complete() {
		return ForwardingAuthorization{}, release.ErrTrustUnavailable
	}
	a, err := authenticateForwarding(ctx, v, c, f, input)
	if err != nil {
		return ForwardingAuthorization{}, err
	}
	return ForwardingAuthorization{pair: a}, nil
}

func authenticateForwarding(ctx context.Context, v *release.Verifier, files inventory, f enrollment.InfrastructureFacts, in release.Inputs) (Authorization, error) {
	if ctx == nil || v == nil || f.Platform != "linux-amd64" || f.Invocation != "forwarding" || in.TargetPath != ProgramTarget || in.Local.Platform != f.Platform || in.Local.Architecture != "amd64" || in.Local.RefTime.IsZero() {
		return Authorization{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return Authorization{}, err
	}
	if err := files.Check(); err != nil {
		return Authorization{}, err
	}
	program, _ := files.File(ProgramName)
	template, _ := files.File(ForwardingTemplateName)
	descriptor, _ := files.File(ForwardingDescriptorName)
	root, _ := files.File("root.json")
	d, err := ValidateForwardingResources(descriptor, program, template)
	if err != nil {
		return Authorization{}, err
	}
	return authenticateBoundPair(ctx, v, files, f, in, d, program, template, descriptor, root, ForwardingGenerationTarget)
}
