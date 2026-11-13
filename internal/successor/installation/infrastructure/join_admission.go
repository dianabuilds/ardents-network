//go:build linux

package infrastructure

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// JoinAuthorization retains a genuine exact Node/JOIN pair. Only the initial
// constructor can retain independently verified initial-pin provenance.
// Its zero value grants nothing. Source/Forwarding proofs, copied receipts and
// public descriptor facts cannot construct it. Native installation and startup
// still require separate original admission.
type JoinAuthorization struct{ pair Authorization }

func (a JoinAuthorization) Check(ctx context.Context, v *release.Verifier) error {
	return a.pair.Check(ctx, v)
}

// Targets returns the retained exact pair for the original start decision.
func (a JoinAuthorization) Targets() (release.Authorization, release.Authorization) {
	return a.pair.Targets()
}

func (a JoinAuthorization) Descriptor() []byte { return a.pair.Descriptor() }
func (a JoinAuthorization) Program() []byte    { return a.pair.Program() }
func (a JoinAuthorization) Template() []byte   { return a.pair.Template() }
func (a JoinAuthorization) InitialFacts() (enrollment.InfrastructureFacts, bool) {
	return a.pair.InitialFacts()
}

// Artifacts copies the closed original inventory. These bytes grant neither
// initial provenance nor native effect permission independently of this owner.
func (a JoinAuthorization) Artifacts() map[string][]byte {
	if a.pair.origin == nil {
		return nil
	}
	files := make(map[string][]byte, 8)
	for _, name := range []string{ProgramName, JoinTemplateName, JoinDescriptorName, "infrastructure-enrollment.json", "root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		raw, ok := a.pair.origin.File(name)
		if !ok {
			return nil
		}
		files[name] = raw
	}
	return files
}

// AuthenticateJoinInitial consumes genuine running-program Enrollment and an
// entirely unborn Release history. Candidate metadata cannot supply this pin.
func AuthenticateJoinInitial(ctx context.Context, v *release.Verifier, b enrollment.InfrastructureBundle, in release.Inputs) (JoinAuthorization, error) {
	f, ok := b.Facts()
	if !ok || f.Invocation != "join" || ctx == nil || v == nil {
		return JoinAuthorization{}, ErrInput
	}
	floors, err := v.CurrentFloors(ctx)
	if err != nil {
		return JoinAuthorization{}, err
	}
	if floors.RootVersion != 0 || floors.TimestampVersion != 0 || floors.SnapshotVersion != 0 || floors.TargetsVersion != 0 || len(floors.RootDigest) != 0 || len(floors.TimestampDigest) != 0 || len(floors.SnapshotDigest) != 0 || len(floors.TargetsDigest) != 0 {
		return JoinAuthorization{}, release.ErrTrustUnavailable
	}
	a, err := authenticateJoinPair(ctx, v, b, f, in)
	if err != nil {
		return JoinAuthorization{}, err
	}
	decision, accepted := a.program.AcceptedDecision()
	if !accepted || decision.Outcome != release.OutcomeReleaseAccepted {
		return JoinAuthorization{}, release.ErrTrustUnavailable
	}
	a.initial = b
	return JoinAuthorization{pair: a}, nil
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
