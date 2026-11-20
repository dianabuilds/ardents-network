//go:build linux

package infrastructure

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// DescriptorAuthorization privately retains the exact Node/Descriptor artifact pair.
// It cannot be passed as Source Authorization or reconstructed from public facts.
// The platform-led lifecycle owns installed admission; this candidate pair
// serves the distinct forward-Release predecessor verification only. The former
// initial admission entry point and detached inventory accessors retired with
// the transitional Descriptor orchestration (#604 FJD stage D, ADR-0125).
type DescriptorAuthorization struct{ pair Authorization }

func (a DescriptorAuthorization) Check(ctx context.Context, v *release.Verifier) error {
	return a.pair.Check(ctx, v)
}

// AuthenticateDescriptorCandidate consumes complete retained Release floors and one
// immutable Descriptor inventory. Canonical Node uses it to verify the exact
// installed payloads under retained floors; it grants no native transition.
func AuthenticateDescriptorCandidate(ctx context.Context, v *release.Verifier, c enrollment.InfrastructureCandidate, in release.Inputs) (DescriptorAuthorization, error) {
	f, ok := c.Facts()
	if !ok || f.Invocation != "descriptor" || ctx == nil || v == nil || f.Platform != "linux-amd64" || in.TargetPath != ProgramTarget || in.Local.Platform != f.Platform || in.Local.Architecture != "amd64" || in.Local.RefTime.IsZero() {
		return DescriptorAuthorization{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return DescriptorAuthorization{}, err
	}
	floors, err := v.CurrentFloors(ctx)
	if err != nil {
		return DescriptorAuthorization{}, err
	}
	if !floors.Complete() {
		return DescriptorAuthorization{}, release.ErrTrustUnavailable
	}
	a, err := authenticateDescriptorPair(ctx, v, c, f, in)
	if err != nil {
		return DescriptorAuthorization{}, err
	}
	return DescriptorAuthorization{pair: a}, nil
}

func authenticateDescriptorPair(ctx context.Context, v *release.Verifier, files inventory, f enrollment.InfrastructureFacts, in release.Inputs) (Authorization, error) {
	if ctx == nil || v == nil || f.Invocation != "descriptor" || f.Platform != "linux-amd64" || in.TargetPath != ProgramTarget || in.Local.Platform != f.Platform || in.Local.Architecture != "amd64" || in.Local.RefTime.IsZero() {
		return Authorization{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return Authorization{}, err
	}
	if err := files.Check(); err != nil {
		return Authorization{}, err
	}
	program, _ := files.File(ProgramName)
	template, _ := files.File(DescriptorTemplateName)
	descriptor, _ := files.File(DescriptorArtifactName)
	root, _ := files.File("root.json")
	d, err := ValidateDescriptorResources(descriptor, program, template)
	if err != nil {
		return Authorization{}, err
	}
	return authenticateBoundPair(ctx, v, files, f, in, d, program, template, descriptor, root, DescriptorGenerationTarget)
}
