package infrastructure

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// IntroductionAuthorization retains a genuine exact Node/Introduction pair. Only the initial
// constructor can retain independently verified initial-pin provenance.
// Its zero value grants nothing. Source/Forwarding proofs, copied receipts and
// public introduction facts cannot construct it. Native installation and startup
// still require separate original admission.
type IntroductionAuthorization struct{ pair Authorization }

func (a IntroductionAuthorization) Check(ctx context.Context, v *release.Verifier) error {
	return a.pair.Check(ctx, v)
}

// Targets lends only the retained exact proofs; native effects still require Check.
func (a IntroductionAuthorization) Targets() (release.Authorization, release.Authorization) {
	return a.pair.program, a.pair.generation
}

func (a IntroductionAuthorization) Descriptor() []byte { return a.pair.Descriptor() }
func (a IntroductionAuthorization) Program() []byte    { return a.pair.Program() }
func (a IntroductionAuthorization) Template() []byte   { return a.pair.Template() }
func (a IntroductionAuthorization) InitialFacts() (enrollment.InfrastructureFacts, bool) {
	return a.pair.InitialFacts()
}

// Artifacts copies the closed original inventory. These bytes grant neither
// initial provenance nor native effect permission independently of this owner.
func (a IntroductionAuthorization) Artifacts() map[string][]byte {
	if a.pair.origin == nil {
		return nil
	}
	files := make(map[string][]byte, 8)
	for _, name := range []string{ProgramName, IntroductionTemplateName, IntroductionArtifactName, "infrastructure-enrollment.json", "root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		raw, ok := a.pair.origin.File(name)
		if !ok {
			return nil
		}
		files[name] = raw
	}
	return files
}

// AuthenticateIntroductionInitial consumes genuine running-program Enrollment and an
// entirely unborn Release history. Candidate metadata cannot supply this pin.
func AuthenticateIntroductionInitial(ctx context.Context, v *release.Verifier, b enrollment.InfrastructureBundle, in release.Inputs) (IntroductionAuthorization, error) {
	a, err := authenticateInitial(ctx, v, b, in, artifactProfile{"introduction", IntroductionTemplateName, IntroductionArtifactName, IntroductionGenerationTarget})
	if err != nil {
		return IntroductionAuthorization{}, err
	}
	return IntroductionAuthorization{pair: a}, nil
}

// AuthenticateIntroductionCandidate consumes complete retained Release floors and one
// immutable Introduction inventory. Canonical Node uses it to verify the exact
// installed payloads under retained floors; it grants no native transition.
func AuthenticateIntroductionCandidate(ctx context.Context, v *release.Verifier, c enrollment.InfrastructureCandidate, in release.Inputs) (IntroductionAuthorization, error) {
	a, err := authenticateStrictCandidate(ctx, v, c, in, artifactProfile{"introduction", IntroductionTemplateName, IntroductionArtifactName, IntroductionGenerationTarget})
	if err != nil {
		return IntroductionAuthorization{}, err
	}
	return IntroductionAuthorization{pair: a}, nil
}
