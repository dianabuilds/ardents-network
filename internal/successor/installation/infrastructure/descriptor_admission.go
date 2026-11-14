//go:build linux

package infrastructure

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// DescriptorAuthorization retains a genuine exact Node/Descriptor pair. Only the initial
// constructor can retain independently verified initial-pin provenance.
// Its zero value grants nothing. Source/Forwarding proofs, copied receipts and
// public descriptor facts cannot construct it. Native installation and startup
// still require separate original admission.
type DescriptorAuthorization struct{ pair Authorization }

func (a DescriptorAuthorization) Check(ctx context.Context, v *release.Verifier) error {
	return a.pair.Check(ctx, v)
}

func (a DescriptorAuthorization) Descriptor() []byte { return a.pair.Descriptor() }
func (a DescriptorAuthorization) Program() []byte    { return a.pair.Program() }
func (a DescriptorAuthorization) Template() []byte   { return a.pair.Template() }
func (a DescriptorAuthorization) InitialFacts() (enrollment.InfrastructureFacts, bool) {
	return a.pair.InitialFacts()
}

// Artifacts copies the closed original inventory. These bytes grant neither
// initial provenance nor native effect permission independently of this owner.
func (a DescriptorAuthorization) Artifacts() map[string][]byte {
	if a.pair.origin == nil {
		return nil
	}
	files := make(map[string][]byte, 8)
	for _, name := range []string{ProgramName, DescriptorTemplateName, DescriptorArtifactName, "infrastructure-enrollment.json", "root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		raw, ok := a.pair.origin.File(name)
		if !ok {
			return nil
		}
		files[name] = raw
	}
	return files
}

// AuthenticateDescriptorInitial consumes genuine running-program Enrollment and an
// entirely unborn Release history. Candidate metadata cannot supply this pin.
func AuthenticateDescriptorInitial(ctx context.Context, v *release.Verifier, b enrollment.InfrastructureBundle, in release.Inputs) (DescriptorAuthorization, error) {
	f, ok := b.Facts()
	if !ok || f.Invocation != "descriptor" || ctx == nil || v == nil {
		return DescriptorAuthorization{}, ErrInput
	}
	floors, err := v.CurrentFloors(ctx)
	if err != nil {
		return DescriptorAuthorization{}, err
	}
	if floors.RootVersion != 0 || floors.TimestampVersion != 0 || floors.SnapshotVersion != 0 || floors.TargetsVersion != 0 || len(floors.RootDigest) != 0 || len(floors.TimestampDigest) != 0 || len(floors.SnapshotDigest) != 0 || len(floors.TargetsDigest) != 0 {
		return DescriptorAuthorization{}, release.ErrTrustUnavailable
	}
	a, err := authenticateDescriptorPair(ctx, v, b, f, in)
	if err != nil {
		return DescriptorAuthorization{}, err
	}
	decision, accepted := a.program.AcceptedDecision()
	if !accepted || decision.Outcome != release.OutcomeReleaseAccepted {
		return DescriptorAuthorization{}, release.ErrTrustUnavailable
	}
	a.initial = b
	return DescriptorAuthorization{pair: a}, nil
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
