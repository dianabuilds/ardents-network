package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The successor State owner selects only the closed Route profile and refuses
// the frozen retired-profile Epoch even when the pinned closed-profile authority
// is supplied. This mirrors the State owner's own frozen-retired refusal oracle
// and proves the acceptance adapter wires that contract rather than publishing a
// retired generation. Genuine closed-Route acceptance is exercised by the State
// owner tests and the two-host qualification preparation consumer.
func TestAcceptOfflineCommandRefusesFrozenRetiredProfile(t *testing.T) {
	t.Parallel()
	base := filepath.Join("..", "..", "internal", "network", "state", "testdata")
	fixture := t.TempDir()
	inputs := filepath.Join(fixture, "inputs")
	if err := os.Mkdir(inputs, 0o700); err != nil {
		t.Fatal(err)
	}
	epoch := writeCommandGolden(t, fixture, "epoch.bin", filepath.Join(base, "epoch.hex"))
	material := writeCommandGolden(t, fixture, "material.bin", filepath.Join(base, "materialization-0000.hex"))
	for index := range 8 {
		writeCommandGolden(t, inputs, fmt.Sprintf("%04d.bin", index), filepath.Join(base, fmt.Sprintf("input-%04d.hex", index)))
	}
	stateRoot := filepath.Join(fixture, "state")
	var output bytes.Buffer
	err := run(t.Context(), []string{
		"accept-offline",
		"-state-root", stateRoot,
		"-network-id", "488a631a444652b50d760a739c338d5f7e54bc14e92a3c3d6002eaeead4f2d3d",
		"-authorities", "c2f38d34dafe402561da5a0a278e8a3255e0fc9c2e58c0209966a589fd07b631",
		"-threshold", "1",
		"-at", time.Unix(1_800_000_100, 0).UTC().Format(time.RFC3339),
		"-epoch", epoch,
		"-inputs", inputs,
		"-materialization", material,
		"-profile", "ardents-route-v3",
		"-closed-profile-authority", "c2f38d34dafe402561da5a0a278e8a3255e0fc9c2e58c0209966a589fd07b631",
	}, &output)
	if err == nil || output.Len() != 0 {
		t.Fatalf("accept-offline published the frozen retired profile: err=%v output=%q", err, output.String())
	}
}

func TestEndpointRouteRejectsIncompleteCommand(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	if err := run(t.Context(), []string{"endpoint", "run"}, &output); err == nil || output.Len() != 0 {
		t.Fatalf("incomplete endpoint command err=%v output=%q", err, output.String())
	}
}

func TestEndpointRouteRejectsRetiredPortableBypass(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	if err := run(t.Context(), []string{"endpoint", "portable"}, &output); err == nil || output.Len() != 0 {
		t.Fatalf("retired portable endpoint route err=%v output=%q", err, output.String())
	}
}

func TestRefreshSourcesRouteReturnsRetirementRefusal(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	err := run(t.Context(), []string{"refresh-sources", "--once", "--resume",
		"--state-root", t.TempDir(), "--source-plan", filepath.Join(t.TempDir(), "absent.json")}, &output)
	if err == nil || err.Error() != errRefreshSourcesRetired.Error() || output.Len() != 0 {
		t.Fatalf("retired refresh-sources err=%v output=%q", err, output.String())
	}
}

func TestEntryImportRouteReturnsRetirementRefusal(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	if err := run(t.Context(), []string{"entry", "import"}, &output); err == nil ||
		err.Error() != errEntryCommandRetired.Error() || output.Len() != 0 {
		t.Fatalf("retired entry command err=%v output=%q", err, output.String())
	}
}

func TestNameRouteRejectsIncompleteCommand(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	if err := run(t.Context(), []string{"name", "encode"}, &output); err == nil || output.Len() != 0 {
		t.Fatalf("incomplete name command err=%v output=%q", err, output.String())
	}
}

func TestRootUsageListsRetainedRoutes(t *testing.T) {
	t.Parallel()
	err := run(t.Context(), nil, &bytes.Buffer{})
	if err == nil || err.Error() != "usage: ardents <accept-offline|accept-closed-profile|diagnostics|endpoint> arguments" {
		t.Fatalf("root usage error = %v", err)
	}
}

func TestServiceInstanceRetiredBeforeEffects(t *testing.T) {
	t.Parallel()
	for _, arguments := range [][]string{
		{"service-instance", "initialize", "--config", "missing.json"},
		{"service-instance", "accept", "--root", "missing", "--response", "missing"},
	} {
		var output bytes.Buffer
		err := run(t.Context(), arguments, &output)
		if err == nil || !strings.Contains(err.Error(), "service-instance is retired") {
			t.Fatalf("retired service-instance reached effects: %v", err)
		}
		if output.Len() != 0 {
			t.Fatalf("retired service-instance wrote output: %s", output.String())
		}
	}
}

func TestAcceptClosedProfileRejectsOfflineEpochInput(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	err := run(t.Context(), []string{"accept-closed-profile", "--epoch", "epoch.bin"}, &output)
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") || output.Len() != 0 {
		t.Fatalf("closed-profile command accepted offline Epoch input: %v / %q", err, output.String())
	}
}

func writeCommandGolden(t *testing.T, directory, name, source string) string {
	t.Helper()
	encoded, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := hex.DecodeString(string(bytes.TrimSpace(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
