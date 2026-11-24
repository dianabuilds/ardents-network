package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
	generationauthorization "github.com/dianabuilds/ardents-network/internal/release/endpoint"
	endpointfixture "github.com/dianabuilds/ardents-network/tests/e2e/endpoint"
	nodefixture "github.com/dianabuilds/ardents-network/tests/e2e/node"
)

// The fixture signs actual compiled consumer bytes with ephemeral test keys.
// Its inventory and custom fields are encoded independently of product codecs.
func signedConsumerProfile(t *testing.T, protected bool) (string, string, string) {
	platform := runtime.GOOS + "-" + runtime.GOARCH
	if protected {
		platform = "linux-amd64"
	}
	return signedConsumerPlatform(t, platform, protected)
}

func signedConsumerPlatform(t *testing.T, platform string, protected bool) (string, string, string) {
	return signedConsumerTargets(t, platform, protected, false, 1)
}

func signedConsumerTargets(t *testing.T, platform string, protected, generation bool, generationVersion int64) (string, string, string) {
	return signedConsumerResources(t, platform, protected, generation, generationVersion, nil)
}

func signedConsumerResources(t *testing.T, platform string, protected, generation bool, generationVersion int64, resources map[string][]byte) (string, string, string) {
	return signedConsumerRelease(t, platform, protected, generation, generationVersion, resources, nil, 1, 1)
}

func signedConsumerRelease(t *testing.T, platform string, protected, generation bool, generationVersion int64, resources map[string][]byte, authority *nodefixture.ReleaseAuthority, metadataVersion, releaseVersion int64, legacyCustody ...[]byte) (string, string, string) {
	t.Helper()
	return endpointfixture.SignedRelease(t, compiledCommand(t), platform, protected, generation, generationVersion, resources, authority, metadataVersion, releaseVersion, legacyCustody...)
}

func TestReleaseInitialProtectedInventoryUsesOnlyMetadata(t *testing.T) {
	directory, pin, program := signedConsumerProfile(t, true)
	bundle, err := enrollment.Verify(t.Context(), enrollment.Request{BundleRoot: directory, ExecutablePath: program, ManifestSHA256: pin})
	if err != nil {
		t.Fatal(err)
	}
	facts, ok := bundle.Facts()
	if !ok || !facts.Protected || len(bundle.Names()) != 15 {
		t.Fatal("fixture lacks genuinely verified protected inventory")
	}
	input, ok := generationauthorization.InitialInputs(bundle, time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC))
	// This metadata-only fixture models the protected linux-amd64 artifact,
	// not execution of that artifact on the current developer host.
	input.Local.Architecture = "amd64"
	if !ok {
		t.Fatal("verified inventory projection refused")
	}
	verifier, err := release.Open(filepath.Join(t.TempDir(), "history"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := verifier.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, expected := range []release.Outcome{release.OutcomeReleaseAccepted, release.OutcomeNoUpdate} {
		decision := verifier.Evaluate(t.Context(), input)
		if _, ok := decision.Authorization(); !ok || decision.Outcome != expected {
			t.Fatalf("genuine protected bundle refused: %s: %v", decision.Outcome, decision.Err())
		}
	}
	if len(input.Files) != 3 {
		t.Fatalf("static inventory escaped into metadata: got %d metadata files, want 3", len(input.Files))
	}
	for _, name := range []string{"timestamp.json", "1.snapshot.json", "1.targets.json"} {
		if _, ok := input.Files["https://release.invalid/metadata/"+name]; !ok {
			t.Fatalf("missing actual metadata %s", name)
		}
	}
}

func TestReleaseCompiledForeignPlatformRefusesBeforeHistory(t *testing.T) {
	foreign := "windows-amd64"
	if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
		// The host must be able to execute the fixture before it can refuse
		// the enrolled platform. Change the declared CPU but keep .exe.
		foreign = "windows-arm64"
	}
	directory, pin, program := signedConsumerPlatform(t, foreign, false)
	history := filepath.Join(t.TempDir(), "absent-history")
	cmd := exec.CommandContext(t.Context(), program, "release", "verify-initial", directory, pin, history, "2030-01-02T03:04:05Z")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "release-incompatible") {
		t.Fatalf("signed foreign platform was not refused: %v: %s", err, out)
	}
	if _, err := os.Lstat(history); !os.IsNotExist(err) {
		t.Fatalf("foreign platform opened history: %v", err)
	}
}

func TestReleaseCompiledInitialVerificationAndRetainedRetry(t *testing.T) {
	t.Run("general", func(t *testing.T) { verifyCompiledReleaseProfile(t, false) })
	// Actual protected executable invocation is selected only where the
	// current linux-amd64 artifact contract applies. Its portable projection
	// is independently exercised on every native test platform above.
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		t.Run("protected", func(t *testing.T) { verifyCompiledReleaseProfile(t, true) })
	}
}

func verifyCompiledReleaseProfile(t *testing.T, protected bool) {
	t.Helper()
	directory, pin, program := signedConsumerProfile(t, protected)
	history := filepath.Join(t.TempDir(), "release-history")
	for _, wanted := range []string{"release-accepted", "no-update"} {
		cmd := exec.CommandContext(t.Context(), program, "release", "verify-initial", directory, pin, history, "2030-01-02T03:04:05Z")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("compiled consumer refused %v: %s", err, out)
		}
		var result struct {
			Outcome     string `json:"outcome"`
			RootVersion int64  `json:"root_version"`
		}
		if err = json.Unmarshal(out, &result); err != nil || result.Outcome != wanted || result.RootVersion != 1 {
			t.Fatalf("actual consumer result: %s", out)
		}
	}
	cmd := exec.CommandContext(t.Context(), program, "release", "verify-initial", directory, strings.Repeat("0", 64), history, "2030-01-02T03:04:05Z")
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "enrollment-refused") {
		t.Fatalf("wrong first pin accepted: %s", out)
	}
}
