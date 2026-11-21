//go:build linux

package infrastructure

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
)

var (
	ErrSourceReleasePaths     = errors.New("infrastructure: Source distribution and history paths must be independent")
	ErrSourceReleaseInventory = errors.New("infrastructure: Source inventory refused")
	ErrSourceReleaseArtifact  = errors.New("infrastructure: Source artifact refused")
	ErrSourceReleaseMetadata  = errors.New("infrastructure: Source metadata refused")
	ErrSourceReleaseHistory   = errors.New("infrastructure: Source Release history unavailable")
)

// SourceReleaseInspection describes an authenticated Source Release after the
// verifier has closed. It carries no authorization or deployment permission.
type SourceReleaseInspection struct {
	ProgramOutcome, GenerationOutcome               release.Outcome
	ProgramSHA256, DescriptorSHA256, TemplateSHA256 [sha256.Size]byte
}

// InspectInitialSourceRelease authenticates a pinned Source distribution against
// new Release history, using the executing program's original Enrollment proof.
func InspectInitialSourceRelease(ctx context.Context, local release.LocalEnvironment, distribution, pin, history string) (SourceReleaseInspection, error) {
	return inspectSourceRelease(ctx, local, distribution, pin, history, true)
}

// InspectCandidateSourceRelease authenticates a candidate against retained
// Release history. It supplies no initial Enrollment provenance.
func InspectCandidateSourceRelease(ctx context.Context, local release.LocalEnvironment, distribution, history string) (SourceReleaseInspection, error) {
	return inspectSourceRelease(ctx, local, distribution, "", history, false)
}

func inspectSourceRelease(ctx context.Context, local release.LocalEnvironment, distribution, pin, history string, initial bool) (SourceReleaseInspection, error) {
	if !independentSourceReleaseHistory(distribution, history) {
		return SourceReleaseInspection{}, ErrSourceReleasePaths
	}

	var b enrollment.InfrastructureBundle
	var c enrollment.InfrastructureCandidate
	var files inventory
	var facts enrollment.InfrastructureFacts
	var err error
	if initial {
		b, err = enrollment.VerifyInfrastructure(ctx, distribution, pin)
		files = b
		facts, _ = b.Facts()
	} else {
		c, err = enrollment.ReadInfrastructureCandidate(ctx, distribution)
		files = c
		facts, _ = c.Facts()
	}
	if err != nil {
		return SourceReleaseInspection{}, errors.Join(ErrSourceReleaseInventory, err)
	}

	program, _ := files.File(ProgramName)
	descriptor, _ := files.File(DescriptorName)
	template, _ := files.File(TemplateName)
	d, err := ValidateResources(descriptor, program, template)
	if err != nil || d.ReleaseIdentity != facts.ReleaseIdentity || d.ReleaseVersion != facts.ReleaseVersion {
		return SourceReleaseInspection{}, errors.Join(ErrSourceReleaseArtifact, err)
	}
	root, _ := files.File("root.json")
	timestamp, _ := files.File("timestamp.json")
	snapshot, _ := files.File("snapshot.json")
	targets, _ := files.File("targets.json")
	metadata, err := release.StaticMetadataFiles(root, timestamp, snapshot, targets)
	if err != nil {
		return SourceReleaseInspection{}, errors.Join(ErrSourceReleaseMetadata, err)
	}
	input := release.Inputs{RootBytes: root, Artifact: program, TargetPath: ProgramTarget, Local: local, Files: metadata}

	var v *release.Verifier
	if initial {
		v, err = release.Open(history)
	} else {
		v, err = release.OpenRetained(history)
	}
	if err != nil {
		return SourceReleaseInspection{}, errors.Join(ErrSourceReleaseHistory, err)
	}
	var a Authorization
	if initial {
		a, err = AuthenticateInitial(ctx, v, b, input)
	} else {
		a, err = AuthenticateCandidate(ctx, v, c, input)
	}
	if err == nil {
		err = a.Check(ctx, v)
	}
	p, g := a.Targets()
	pd, pok := p.AcceptedDecision()
	gd, gok := g.AcceptedDecision()
	_, hasInitial := a.InitialFacts()
	if err == nil && (!pok || !gok || hasInitial != initial) {
		err = ErrAuthorization
	}
	if err := errors.Join(err, v.Close(), ctx.Err()); err != nil {
		return SourceReleaseInspection{}, err
	}
	return SourceReleaseInspection{
		ProgramOutcome:    pd.Outcome,
		GenerationOutcome: gd.Outcome,
		ProgramSHA256:     sha256.Sum256(a.Program()),
		DescriptorSHA256:  sha256.Sum256(a.Descriptor()),
		TemplateSHA256:    sha256.Sum256(a.Template()),
	}, nil
}

// Check lexical independence and refuse a symlinked history ancestor before
// Release can take its native lease or create a history directory.
func independentSourceReleaseHistory(distribution, history string) bool {
	if !filepath.IsAbs(distribution) || filepath.Clean(distribution) != distribution ||
		!filepath.IsAbs(history) || filepath.Clean(history) != history {
		return false
	}
	contains := func(parent, child string) bool {
		rel, err := filepath.Rel(parent, child)
		return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
	}
	if contains(distribution, history) || contains(history, distribution) {
		return false
	}
	for p := history; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil && !os.IsNotExist(err) {
			return false
		}
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return false
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return true
}
