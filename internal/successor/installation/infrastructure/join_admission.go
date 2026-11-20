//go:build linux

package infrastructure

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// JoinAuthorization privately retains the exact Node/JOIN artifact pair.
// It cannot be passed as Source Authorization or reconstructed from public facts.
// The platform-led lifecycle owns installed admission; this candidate pair
// serves the distinct forward-Release predecessor verification only. The former
// initial admission entry point and detached inventory accessors retired with
// the transitional JOIN orchestration (#604 FJD stage D, ADR-0125).
type JoinAuthorization struct{ pair Authorization }

func (a JoinAuthorization) Check(ctx context.Context, v *release.Verifier) error {
	return a.pair.Check(ctx, v)
}

// AuthenticateJoinCandidate consumes complete retained Release floors and one
// immutable JOIN inventory. Canonical Control uses it to verify the exact
// predecessor before forward signing; it grants no installed transition.
func AuthenticateJoinCandidate(ctx context.Context, v *release.Verifier, c enrollment.InfrastructureCandidate, in release.Inputs) (JoinAuthorization, error) {
	f, ok := c.Facts()
	if !ok || f.Invocation != "join" || ctx == nil || v == nil || f.Platform != "linux-amd64" || in.TargetPath != ProgramTarget || in.Local.Platform != f.Platform || in.Local.Architecture != "amd64" || in.Local.RefTime.IsZero() {
		return JoinAuthorization{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return JoinAuthorization{}, err
	}
	floors, err := v.CurrentFloors(ctx)
	if err != nil {
		return JoinAuthorization{}, err
	}
	if !floors.Complete() {
		return JoinAuthorization{}, release.ErrTrustUnavailable
	}
	a, err := authenticateJoinPair(ctx, v, c, f, in)
	if err != nil {
		return JoinAuthorization{}, err
	}
	return JoinAuthorization{pair: a}, nil
}

func authenticateJoinPair(ctx context.Context, v *release.Verifier, files inventory, f enrollment.InfrastructureFacts, in release.Inputs) (Authorization, error) {
	if ctx == nil || v == nil || f.Invocation != "join" || f.Platform != "linux-amd64" || in.TargetPath != ProgramTarget || in.Local.Platform != f.Platform || in.Local.Architecture != "amd64" || in.Local.RefTime.IsZero() {
		return Authorization{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return Authorization{}, err
	}
	if err := files.Check(); err != nil {
		return Authorization{}, err
	}
	program, _ := files.File(ProgramName)
	template, _ := files.File(JoinTemplateName)
	descriptor, _ := files.File(JoinDescriptorName)
	root, _ := files.File("root.json")
	d, err := ValidateJoinResources(descriptor, program, template)
	if err != nil {
		return Authorization{}, err
	}
	return authenticateBoundPair(ctx, v, files, f, in, d, program, template, descriptor, root, JoinGenerationTarget)
}
