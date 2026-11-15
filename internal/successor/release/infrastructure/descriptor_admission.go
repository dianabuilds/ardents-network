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

// Targets lends only this exact pair's original Release authorizations to the
// root start transcript; copies cannot recreate permission or initial pinning.
func (a DescriptorAuthorization) Targets() (release.Authorization, release.Authorization) {
	return a.pair.Targets()
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
	a, err := authenticateInitial(ctx, v, b, in, artifactProfile{"descriptor", DescriptorTemplateName, DescriptorArtifactName, DescriptorGenerationTarget})
	if err != nil {
		return DescriptorAuthorization{}, err
	}
	return DescriptorAuthorization{pair: a}, nil
}

// AuthenticateDescriptorCandidate consumes complete retained Release floors and one
// immutable Descriptor inventory. Canonical Node uses it to verify the exact
// installed payloads under retained floors; it grants no native transition.
func AuthenticateDescriptorCandidate(ctx context.Context, v *release.Verifier, c enrollment.InfrastructureCandidate, in release.Inputs) (DescriptorAuthorization, error) {
	a, err := authenticateStrictCandidate(ctx, v, c, in, artifactProfile{"descriptor", DescriptorTemplateName, DescriptorArtifactName, DescriptorGenerationTarget})
	if err != nil {
		return DescriptorAuthorization{}, err
	}
	return DescriptorAuthorization{pair: a}, nil
}
