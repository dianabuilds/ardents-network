//go:build linux

package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Authorization retains exact immutable bytes, original initial provenance when
// applicable, and genuine private Release results. Its zero value grants nothing.
// This component grants no installed native effect or startup permission.
type Authorization struct {
	origin                           inventory
	initial                          enrollment.InfrastructureBundle
	program, generation              release.Authorization
	descriptor, executable, template []byte
}

// Check revalidates the original immutable snapshot/caller and Release owner
// before this component's handoff. Native Installation must additionally admit
// its own exact current resources and effects; this Check grants none of them.
func (a Authorization) Check(ctx context.Context, v *release.Verifier) error {
	if ctx == nil || v == nil || a.origin == nil {
		return ErrInput
	}
	if err := a.origin.Check(); err != nil {
		return err
	}
	if err := v.CheckAuthorizations(ctx, a.program, a.generation); err != nil {
		return err
	}
	return errors.Join(a.origin.Check(), ctx.Err())
}

func (a Authorization) Targets() (release.Authorization, release.Authorization) {
	return a.program, a.generation
}
func (a Authorization) Descriptor() []byte { return bytes.Clone(a.descriptor) }
func (a Authorization) Program() []byte    { return bytes.Clone(a.executable) }
func (a Authorization) Template() []byte   { return bytes.Clone(a.template) }
func (a Authorization) InitialFacts() (enrollment.InfrastructureFacts, bool) {
	return a.initial.Facts()
}

func AuthenticateInitial(ctx context.Context, v *release.Verifier, b enrollment.InfrastructureBundle, input release.Inputs) (Authorization, error) {
	f, ok := b.Facts()
	if !ok || f.Invocation != "source" || ctx == nil || v == nil {
		return Authorization{}, ErrInput
	}
	floors, err := v.CurrentFloors(ctx)
	if err != nil {
		return Authorization{}, err
	}
	if floors.RootVersion != 0 || floors.TimestampVersion != 0 || floors.SnapshotVersion != 0 || floors.TargetsVersion != 0 || len(floors.RootDigest) != 0 || len(floors.TimestampDigest) != 0 || len(floors.SnapshotDigest) != 0 || len(floors.TargetsDigest) != 0 {
		return Authorization{}, release.ErrTrustUnavailable
	}
	a, err := authenticate(ctx, v, b, f, input)
	if err == nil {
		// A concurrent first evaluation can establish history after the empty
		// observation; no-update then supplies candidate authority only.
		p, valid := a.program.AcceptedDecision()
		if !valid || p.Outcome != release.OutcomeReleaseAccepted {
			return Authorization{}, release.ErrTrustUnavailable
		}
		a.initial = b
	}
	return a, err
}

func AuthenticateCandidate(ctx context.Context, v *release.Verifier, c enrollment.InfrastructureCandidate, input release.Inputs) (Authorization, error) {
	f, ok := c.Facts()
	if !ok || f.Invocation != "source" || ctx == nil || v == nil {
		return Authorization{}, ErrInput
	}
	floors, err := v.CurrentFloors(ctx)
	if err != nil {
		return Authorization{}, err
	}
	if !floors.Complete() {
		return Authorization{}, release.ErrTrustUnavailable
	}
	return authenticate(ctx, v, c, f, input)
}

type inventory interface {
	File(string) ([]byte, bool)
	Check() error
}

func authenticate(ctx context.Context, v *release.Verifier, files inventory, f enrollment.InfrastructureFacts, in release.Inputs) (Authorization, error) {
	if ctx == nil || v == nil || f.Platform != "linux-amd64" || f.Invocation != "source" || in.TargetPath != ProgramTarget || in.Local.Platform != f.Platform || in.Local.Architecture != "amd64" || in.Local.RefTime.IsZero() {
		return Authorization{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return Authorization{}, err
	}
	if err := files.Check(); err != nil {
		return Authorization{}, err
	}
	program, _ := files.File(ProgramName)
	template, _ := files.File(TemplateName)
	descriptor, _ := files.File(DescriptorName)
	root, _ := files.File("root.json")
	d, err := ValidateResources(descriptor, program, template)
	if err != nil {
		return Authorization{}, err
	}
	return authenticateBoundPair(ctx, v, files, f, in, d, program, template, descriptor, root, GenerationTarget)
}

// This common immutable-byte/Release operation admits no native role policy.
// Its two callers independently validate their closed invocation/resources;
// only their distinct public authorization types can reach native consumers.
func authenticateBoundPair(ctx context.Context, v *release.Verifier, files inventory, f enrollment.InfrastructureFacts, in release.Inputs, d Declaration, program, template, descriptor, root []byte, target string) (Authorization, error) {
	if d.Invocation != f.Invocation || d.ReleaseIdentity != f.ReleaseIdentity || d.ReleaseVersion != f.ReleaseVersion || !bytes.Equal(root, in.RootBytes) || !bytes.Equal(program, in.Artifact) {
		return Authorization{}, ErrBinding
	}
	ts, _ := files.File("timestamp.json")
	sn, _ := files.File("snapshot.json")
	targets, _ := files.File("targets.json")
	metadata, err := release.StaticMetadataFiles(root, ts, sn, targets)
	if err != nil || len(metadata) != len(in.Files) {
		return Authorization{}, errors.Join(ErrBinding, err)
	}
	for location, supplied := range in.Files {
		original, ok := metadata[location]
		if !ok || !bytes.Equal(original, supplied) {
			return Authorization{}, ErrBinding
		}
	}
	// Freeze all caller data before either evaluation; a first committed floor
	// never refunds when the second target or final pair refuses.
	in.RootBytes, in.Artifact, in.Files = root, program, metadata
	p := v.Evaluate(ctx, in)
	pp, ok := p.Authorization()
	if !ok {
		return Authorization{}, errors.Join(ErrAuthorization, fmt.Errorf("program: %s", p.Outcome), p.Err())
	}
	in.TargetPath, in.Artifact = target, descriptor
	g := v.Evaluate(ctx, in)
	gp, ok := g.Authorization()
	if !ok {
		return Authorization{}, errors.Join(ErrAuthorization, fmt.Errorf("generation: %s", g.Outcome), g.Err())
	}
	pd, pok := pp.AcceptedDecision()
	gd, gok := gp.AcceptedDecision()
	ps, gs := sha256.Sum256(program), sha256.Sum256(descriptor)
	if !pok || !gok || !coherentTarget(pd, gd, d, target) || !bytes.Equal(pd.Digest, ps[:]) || !bytes.Equal(gd.Digest, gs[:]) {
		return Authorization{}, ErrBinding
	}
	// Release owns original closure/cancellation/current-floor admission. Public
	// Decisions and serialized receipts cannot supply these private proofs.
	if err := v.CheckAuthorizations(ctx, pp, gp); err != nil {
		return Authorization{}, err
	}
	if err := files.Check(); err != nil {
		return Authorization{}, err
	}
	return Authorization{origin: files, program: pp, generation: gp, descriptor: descriptor, executable: program, template: template}, nil
}

func coherentTarget(p, g release.Decision, d Declaration, target string) bool {
	return p.Path == ProgramTarget && g.Path == target && p.ReleaseIdentity == d.ReleaseIdentity && g.ReleaseIdentity == d.ReleaseIdentity && p.ReleaseVersion == d.ReleaseVersion && g.ReleaseVersion == d.ReleaseVersion &&
		p.Platform == d.Platform && g.Platform == d.Platform && p.Architecture == g.Architecture && p.Environment == g.Environment && p.Network == g.Network && p.ReferenceTime.Equal(g.ReferenceTime) &&
		p.SourceRevision == g.SourceRevision && p.BuildInputCommitment == g.BuildInputCommitment && p.AttestationPolicy == g.AttestationPolicy && p.Qualification == g.Qualification && p.BuildState == g.BuildState && p.ProtocolPhase == g.ProtocolPhase &&
		p.BuildSafety == g.BuildSafety && p.Protocol == g.Protocol && p.BuildSafetyNoNewWorkAfter.Equal(g.BuildSafetyNoNewWorkAfter) && p.BuildSafetyTerminateAfter.Equal(g.BuildSafetyTerminateAfter) && p.ProtocolTransitionDeadline.Equal(g.ProtocolTransitionDeadline) &&
		p.Floors.TargetsVersion == g.Floors.TargetsVersion && bytes.Equal(p.Floors.TargetsDigest, g.Floors.TargetsDigest)
}
