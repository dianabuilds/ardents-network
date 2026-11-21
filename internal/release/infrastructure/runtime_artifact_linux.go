package infrastructure

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
)

// RuntimeAuthorization binds software for a platform-managed invocation. It is
// deliberately distinct from every native Installation transition permission.
// It cannot authorize account creation, selection, Start, replacement or recovery.
type RuntimeAuthorization struct{ pair Authorization }

func authenticateRuntime(ctx context.Context, v *release.Verifier, files inventory, f enrollment.InfrastructureFacts, in release.Inputs) (RuntimeAuthorization, error) {
	descriptorName, templateName, target, ok := runtimeNames(f.Invocation)
	if !ok || ctx == nil || v == nil || f.Platform != "linux-amd64" || in.TargetPath != ProgramTarget || in.Local.Platform != f.Platform || in.Local.Architecture != "amd64" || in.Local.RefTime.IsZero() {
		return RuntimeAuthorization{}, ErrInput
	}
	if err := files.Check(); err != nil {
		return RuntimeAuthorization{}, err
	}
	program, _ := files.File(ProgramName)
	template, _ := files.File(templateName)
	descriptor, _ := files.File(descriptorName)
	root, _ := files.File("root.json")
	d, err := decodeDescriptor(descriptor, f.Invocation, templateName)
	if err != nil || len(program) == 0 || len(template) == 0 || d.Files[ProgramName] != digest(program) || d.Files[templateName] != digest(template) {
		return RuntimeAuthorization{}, ErrBinding
	}
	a, err := authenticateBoundPair(ctx, v, files, f, in, d, program, template, descriptor, root, target)
	return RuntimeAuthorization{pair: a}, err
}

// AuthenticateRuntimeCandidate requires retained trust. Inventory consistency
// is never an initial pin and missing floors never establish new trust.
func AuthenticateRuntimeCandidate(ctx context.Context, v *release.Verifier, c enrollment.InfrastructureCandidate, in release.Inputs) (RuntimeAuthorization, error) {
	f, ok := c.Facts()
	if !ok || ctx == nil || v == nil {
		return RuntimeAuthorization{}, ErrInput
	}
	floors, err := v.CurrentFloors(ctx)
	if err != nil || !floors.Complete() {
		return RuntimeAuthorization{}, errors.Join(release.ErrTrustUnavailable, err)
	}
	return authenticateRuntime(ctx, v, c, f, in)
}

// AuthenticateRuntimeInitial is used only by explicit offline trust setup, not
// ExecStartPre. Native Installation cannot consume its result.
func AuthenticateRuntimeInitial(ctx context.Context, v *release.Verifier, b enrollment.InfrastructureBundle, in release.Inputs) (RuntimeAuthorization, error) {
	f, ok := b.Facts()
	if !ok || ctx == nil || v == nil {
		return RuntimeAuthorization{}, ErrInput
	}
	floors, err := v.CurrentFloors(ctx)
	if err != nil || floors.RootVersion != 0 || floors.TimestampVersion != 0 || floors.SnapshotVersion != 0 || floors.TargetsVersion != 0 || len(floors.RootDigest) != 0 || len(floors.TimestampDigest) != 0 || len(floors.SnapshotDigest) != 0 || len(floors.TargetsDigest) != 0 {
		return RuntimeAuthorization{}, errors.Join(release.ErrTrustUnavailable, err)
	}
	a, err := authenticateRuntime(ctx, v, b, f, in)
	if err == nil {
		decision, ok := a.pair.program.AcceptedDecision()
		if !ok || decision.Outcome != release.OutcomeReleaseAccepted {
			return RuntimeAuthorization{}, release.ErrTrustUnavailable
		}
	}
	return a, err
}

func (a RuntimeAuthorization) Check(ctx context.Context, v *release.Verifier) error {
	return a.pair.Check(ctx, v)
}
