package installation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	requestinput "github.com/dianabuilds/ardents-network/internal/successor/installation/request"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

var (
	ErrInput         = requestinput.ErrInput
	ErrBinding       = errors.New("installation: generation binding differs")
	ErrAuthorization = errors.New("installation: fresh Release authorization refused")
)

const programTarget = "ardents/linux-amd64/endpoint"
const generationTarget = "ardents/linux-amd64/protected-endpoint"

// Authorization retains two private fresh proofs and their exact generation
// bytes. It is not serializable installation or restart authority. Its zero
// value grants nothing; installed ownership and admission remain separate.
type Authorization struct {
	initial             enrollment.Bundle
	program, generation release.Authorization
	descriptor          []byte
	resources           map[string][]byte
}

func (a Authorization) Targets() (release.Authorization, release.Authorization) {
	return a.program, a.generation
}

// Descriptor and Resource expose detached exact bytes, never reusable authority.
func (a Authorization) Descriptor() []byte { return bytes.Clone(a.descriptor) }

func (a Authorization) Resource(name string) ([]byte, bool) {
	b, ok := a.resources[name]
	return bytes.Clone(b), ok
}

// AuthenticateInitial consumes genuine independent-pin provenance. Composition
// supplies metadata URLs and local facts; every supplied byte must still be
// the snapshot's byte before either Release evaluation can advance floors.
func AuthenticateInitial(ctx context.Context, v *release.Verifier, b enrollment.Bundle, in release.Inputs) (Authorization, error) {
	f, ok := b.Facts()
	if !ok {
		return Authorization{}, ErrInput
	}
	accepted, err := authenticate(ctx, v, b, f, in)
	if err == nil {
		// Preserve genuine initial-pin provenance privately. A candidate pair
		// cannot acquire it by carrying the same public descriptor or Decision.
		accepted.initial = b
	}
	return accepted, err
}

// AuthenticateCandidate requires already established complete Release floors.
// A candidate supplies consistency, never initial trust or an installed binding.
// A future successor transition must additionally check its retained predecessor.
func AuthenticateCandidate(ctx context.Context, v *release.Verifier, c enrollment.Candidate, in release.Inputs) (Authorization, error) {
	f, ok := c.Facts()
	if !ok || ctx == nil || v == nil {
		return Authorization{}, ErrInput
	}
	floors, err := v.CurrentFloors(ctx)
	if err != nil {
		return Authorization{}, err
	}
	if !completeReleaseFloors(floors) {
		return Authorization{}, release.ErrTrustUnavailable
	}
	return authenticate(ctx, v, c, enrollment.Facts(f), in)
}

// These two actual immutable inventory results share byte access, not authority.
type inventory interface {
	File(string) ([]byte, bool)
	MetadataNames() []string
}

type generationDeclaration struct {
	Platform        string            `json:"platform"`
	ReleaseIdentity string            `json:"release_identity"`
	ReleaseVersion  int64             `json:"release_version"`
	Files           map[string]string `json:"files"`
}

func authenticate(ctx context.Context, v *release.Verifier, files inventory, f enrollment.Facts, in release.Inputs) (Authorization, error) {
	if ctx == nil || v == nil || !f.Protected || !f.Headless ||
		f.Platform != "linux-amd64" || f.TargetPath != programTarget ||
		in.TargetPath != programTarget || in.Local.Platform != f.Platform ||
		in.Local.Architecture != "amd64" || in.Local.Environment != f.Environment ||
		in.Local.Network != f.Network || in.Local.RefTime.IsZero() {
		return Authorization{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return Authorization{}, err
	}
	root, rootOK := files.File(f.TrustedRoot)
	program, programOK := files.File(f.Artifact)
	descriptor, descriptorOK := files.File("protected-endpoint.json")
	if !rootOK || !programOK || !descriptorOK ||
		!bytes.Equal(root, in.RootBytes) || !bytes.Equal(program, in.Artifact) {
		return Authorization{}, ErrBinding
	}
	// Decode only already inventory-checked descriptor facts. Enrollment retains
	// canonical grammar, exact resource set and digest validation ownership.
	var declaration generationDeclaration
	if err := json.Unmarshal(descriptor, &declaration); err != nil {
		return Authorization{}, errors.Join(ErrBinding, err)
	}
	expected := make(map[string]bool)
	for _, name := range files.MetadataNames() {
		expected[name] = true
	}
	if len(in.Files) != len(expected) {
		return Authorization{}, ErrBinding
	}
	metadata := make(map[string][]byte, len(expected))
	for location, supplied := range in.Files {
		u, err := url.Parse(location)
		if err != nil {
			return Authorization{}, ErrBinding
		}
		name := path.Base(u.Path)
		original, ok := files.File(name)
		if !expected[name] || !ok || !bytes.Equal(original, supplied) {
			return Authorization{}, ErrBinding
		}
		delete(expected, name)
		metadata[location] = original
	}
	resources := make(map[string][]byte, len(declaration.Files))
	for name := range declaration.Files {
		data, ok := files.File(name)
		if !ok {
			return Authorization{}, ErrBinding
		}
		resources[name] = data
	}
	// Both evaluations use one private immutable input, including reference time.
	// There is no rollback when the first evaluation committed trust history.
	in.RootBytes, in.Artifact, in.Files = root, program, metadata
	p := v.Evaluate(ctx, in)
	programProof, ok := p.Authorization()
	if !ok {
		return Authorization{}, errors.Join(ErrAuthorization, fmt.Errorf("program: %s", p.Outcome), p.Err())
	}
	in.TargetPath, in.Artifact = generationTarget, descriptor
	g := v.Evaluate(ctx, in)
	generationProof, ok := g.Authorization()
	if !ok {
		return Authorization{}, errors.Join(ErrAuthorization, fmt.Errorf("generation: %s", g.Outcome), g.Err())
	}
	acceptedProgram, programOK := programProof.AcceptedDecision()
	acceptedGeneration, generationOK := generationProof.AcceptedDecision()
	if !programOK || !generationOK || !coherentTargets(acceptedProgram, acceptedGeneration, declaration) {
		return Authorization{}, ErrBinding
	}
	programDigest, generationDigest := sha256.Sum256(program), sha256.Sum256(descriptor)
	if !bytes.Equal(acceptedProgram.Digest, programDigest[:]) || !bytes.Equal(acceptedGeneration.Digest, generationDigest[:]) {
		return Authorization{}, ErrBinding
	}
	if err := ctx.Err(); err != nil {
		return Authorization{}, err
	}
	return Authorization{program: programProof, generation: generationProof, descriptor: descriptor, resources: resources}, nil
}

func coherentTargets(p, g release.Decision, d generationDeclaration) bool {
	return p.Path == programTarget && g.Path == generationTarget &&
		p.ReleaseIdentity == d.ReleaseIdentity && g.ReleaseIdentity == d.ReleaseIdentity &&
		p.ReleaseVersion == d.ReleaseVersion && g.ReleaseVersion == d.ReleaseVersion &&
		p.Platform == d.Platform && g.Platform == d.Platform &&
		p.Architecture == g.Architecture && p.Environment == g.Environment &&
		p.Network == g.Network && p.ReferenceTime.Equal(g.ReferenceTime) &&
		p.Floors.TargetsVersion == g.Floors.TargetsVersion &&
		bytes.Equal(p.Floors.TargetsDigest, g.Floors.TargetsDigest)
}

func completeReleaseFloors(f release.FloorSet) bool {
	return f.RootVersion > 0 && len(f.RootDigest) == sha256.Size &&
		f.TimestampVersion > 0 && len(f.TimestampDigest) == sha256.Size &&
		f.SnapshotVersion > 0 && len(f.SnapshotDigest) == sha256.Size &&
		f.TargetsVersion > 0 && len(f.TargetsDigest) == sha256.Size
}

// Local stored facts constrain continuity, never create fresh authorization.
// The caller must supply these facts from its still-leased native inspection.
func successorContinuity(previous generationBinding, floors release.FloorSet, local release.LocalEnvironment) error {
	g := previous.Generation
	if !completeReleaseFloors(floors) || g.TargetsVersion < 1 || !canonicalDigest(g.TargetsDigest) ||
		floors.TargetsVersion < g.TargetsVersion ||
		(floors.TargetsVersion == g.TargetsVersion && hex.EncodeToString(floors.TargetsDigest) != g.TargetsDigest) {
		return release.ErrTrustUnavailable
	}
	if local.Platform != g.Platform || local.Architecture != g.Architecture ||
		local.Environment != g.Environment || local.Network != g.Network {
		return ErrBinding
	}
	before, err := time.Parse(time.RFC3339Nano, g.ReferenceTime)
	if err != nil || local.RefTime.IsZero() || local.RefTime.Before(before) {
		return ErrBinding
	}
	return nil
}
