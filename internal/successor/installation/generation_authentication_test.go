package installation

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Failure-only public-boundary checks provide no successful authority fixture.
// Genuine signed-byte/owned-history success and refusal are exercised by the
// production command integration tests in cmd/ardents-next.
func TestZeroInventoryAndAuthorizationGrantNothing(t *testing.T) {
	if _, err := AuthenticateInitial(context.Background(), nil, enrollment.Bundle{}, release.Inputs{}); !errors.Is(err, ErrInput) {
		t.Fatalf("zero initial inventory: %v", err)
	}
	if _, err := AuthenticateCandidate(context.Background(), nil, enrollment.Candidate{}, release.Inputs{}); !errors.Is(err, ErrInput) {
		t.Fatalf("zero candidate: %v", err)
	}
	p, g := (Authorization{}).Targets()
	if _, ok := p.AcceptedDecision(); ok {
		t.Fatal("zero program authorized")
	}
	if _, ok := g.AcceptedDecision(); ok {
		t.Fatal("zero generation authorized")
	}
}

func TestAssemblyCannotUseStoredObservationsAsFreshAuthority(t *testing.T) {
	_, binding, files := installedObservationFixture(t)
	request, err := DecodeRequest(t.Context(), files["request.json"], true)
	if err != nil {
		t.Fatal(err)
	}
	// A complete, coherent stored binding and exact static bytes still have no
	// private Release proof. Numeric root facts here are public observations,
	// not a successful native preparation or an installed positive fixture.
	publicProgram, err := binding.Program.observation()
	if err != nil {
		t.Fatal(err)
	}
	publicGeneration, err := binding.Generation.observation()
	if err != nil {
		t.Fatal(err)
	}
	if !coherentTargets(publicProgram, publicGeneration, generationDeclaration{
		Platform: "linux-amd64", ReleaseIdentity: "release", ReleaseVersion: 1,
	}) {
		t.Fatal("stored fixture is not coherent")
	}
	var descriptor generationDeclaration
	if err := json.Unmarshal(files["protected-endpoint.json"], &descriptor); err != nil {
		t.Fatal(err)
	}
	resources := make(map[string][]byte, len(descriptor.Files))
	for name := range descriptor.Files {
		resources[name] = files[name]
	}
	if err := enrollment.ValidateProtectedGeneration(files["protected-endpoint.json"], resources, "release"); err != nil {
		t.Fatal(err)
	}
	observed := Authorization{descriptor: files["protected-endpoint.json"], resources: resources}
	prepared := preparedInstallation{uid: binding.UID, gid: binding.GID, roots: binding.MutableRoots}
	assembled, selected, err := assembleGeneration(t.Context(), request, observed, prepared)
	if !errors.Is(err, ErrAuthorization) || assembled != nil || selected != (generationSelection{}) {
		t.Fatalf("stored observations became fresh assembly authority: %v", err)
	}
}
