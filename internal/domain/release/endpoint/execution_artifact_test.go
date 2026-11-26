package endpoint_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/enrollment"
	"github.com/dianabuilds/ardents-network/internal/domain/release"
	endpointrelease "github.com/dianabuilds/ardents-network/internal/domain/release/endpoint"
	releasefixture "github.com/dianabuilds/ardents-network/tests/releasefixture"
)

// Actual program/worker/unit bytes are necessary for installed startup. This
// checks their genuine Enrollment and Release owners directly; no manager,
// root completion peer or accepting worker is substituted by this fixture.
func TestExecutionSignedInstalledResources(t *testing.T) {
	program := buildLinuxReleaseArtifact(t, "ardents", "./cmd/ardents-next")
	worker := buildLinuxReleaseArtifact(t, "ardents-text", "./internal/application/document/worker")
	resources := make(map[string][]byte)
	var err error
	resources["ardents-text-linux-amd64"], err = os.ReadFile(worker)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ardents-text-reader@.service", "ardents-text-publisher@.service", "ardents-text-reader.socket", "ardents-text-publisher.socket", "50-ardents-text.rules", "ardents-text.conf", "ardents-endpoint.service"} {
		resources[name], err = os.ReadFile(filepath.Join("..", "..", "..", "..", "packaging", "document-worker", name))
		if err != nil {
			t.Fatal(err)
		}
	}
	authority := releasefixture.NewReleaseAuthority(t, time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC).Add(365*24*time.Hour))

	bundle, pin, enrolledProgram := releasefixture.SignedRelease(t, program, "linux-amd64", true, true, 1, resources, authority, 1, 1)
	reference := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	history := filepath.Join(t.TempDir(), "floors")
	verified, err := enrollment.Verify(t.Context(), enrollment.Request{BundleRoot: bundle, ExecutablePath: enrolledProgram, ManifestSHA256: pin})
	if err != nil {
		t.Fatal(err)
	}
	input, ok := endpointrelease.InitialInputs(verified, reference)
	input.Local.Architecture = "amd64"
	if !ok {
		t.Fatal("missing initial inputs")
	}
	verifier, err := release.Open(history)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := verifier.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := endpointrelease.AuthenticateInitial(t.Context(), verifier, verified, input); err != nil {
		t.Fatal(err)
	}
	candidateResources := make(map[string][]byte, len(resources))
	for name, body := range resources {
		candidateResources[name] = append([]byte(nil), body...)
	}
	candidateResources["ardents-endpoint.service"] = append([]byte("# signed successor fixture\n"), candidateResources["ardents-endpoint.service"]...)
	candidateRoot, _, _ := releasefixture.SignedRelease(t, program, "linux-amd64", true, true, 2, candidateResources, authority, 2, 2)
	candidate, err := enrollment.ReadCandidate(t.Context(), candidateRoot)
	if err != nil {
		t.Fatal(err)
	}
	next, ok := endpointrelease.CandidateInputs(candidate, reference)
	next.Local.Architecture = "amd64"
	if !ok {
		t.Fatal("missing candidate inputs")
	}
	if _, err := endpointrelease.AuthenticateCandidate(t.Context(), verifier, candidate, next); err != nil {
		t.Fatal(err)
	}
	old, err := enrollment.ReadCandidate(t.Context(), bundle)
	if err != nil {
		t.Fatal(err)
	}
	stale, ok := endpointrelease.CandidateInputs(old, reference)
	stale.Local.Architecture = "amd64"
	if !ok {
		t.Fatal("missing old candidate inputs")
	}
	if _, err := endpointrelease.AuthenticateCandidate(t.Context(), verifier, old, stale); err == nil {
		t.Fatal("accepted old signed snapshot after candidate floors")
	}
	changed := append([]byte(nil), resources["ardents-text-linux-amd64"]...)
	changed[0] ^= 1
	if err := os.WriteFile(filepath.Join(bundle, "ardents-text-linux-amd64"), changed, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := enrollment.Verify(t.Context(), enrollment.Request{BundleRoot: bundle, ExecutablePath: enrolledProgram, ManifestSHA256: pin}); err == nil {
		t.Fatal("accepted substituted actual worker")
	}
}

func buildLinuxReleaseArtifact(t *testing.T, name, packagePath string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), name)
	build := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-w", "-o", binary, packagePath)
	build.Dir = filepath.Join("..", "..", "..", "..")
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v: %s", packagePath, err, out)
	}
	return binary
}
