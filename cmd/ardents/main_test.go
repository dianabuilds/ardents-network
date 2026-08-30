package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAcceptOfflineCommandPublishesFrozenGeneration(t *testing.T) {
	t.Parallel()
	base := "testdata"
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
	}, &output)
	if err != nil {
		t.Fatalf("run accept-offline: %v", err)
	}
	var result struct {
		Schema     string `json:"schema"`
		Generation string `json:"generation"`
		Epoch      uint64 `json:"epoch"`
		ViewLength uint32 `json:"view_length"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if result.Schema != "ardents-state-event-v1" || result.Generation != "243fba444fe71948f6cd4a253552301192857a156c7eb6359eed604c2d2cda4b" || result.Epoch != 1 || result.ViewLength != 2 {
		t.Fatalf("unexpected command result: %+v", result)
	}
	wantEvent, err := os.ReadFile(filepath.Join(base, "event.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), wantEvent) {
		t.Fatalf("event bytes differ:\n got %s\nwant %s", output.Bytes(), wantEvent)
	}
}

func TestEndpointRouteRejectsIncompleteCommand(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	if err := run(t.Context(), []string{"endpoint", "run"}, &output); err == nil || output.Len() != 0 {
		t.Fatalf("incomplete endpoint command err=%v output=%q", err, output.String())
	}
}

func TestPortableUserUnitEscapesExactAbsoluteInputs(t *testing.T) {
	t.Parallel()
	executable := filepath.Join(t.TempDir(), "bin", "ardents")
	enrollment := filepath.Join(t.TempDir(), "alpha $cohort%.json")
	unit, err := portableUserUnit(executable, enrollment)
	if err != nil {
		t.Fatal(err)
	}
	escapedExecutable, _ := unitArgument(executable)
	escapedEnrollment, _ := unitArgument(enrollment)
	if !strings.Contains(unit, "ExecStart="+escapedExecutable+" endpoint enroll "+escapedEnrollment) ||
		!strings.Contains(escapedEnrollment, "$$cohort%%") ||
		!strings.Contains(unit, "UMask=0077\nRestart=no\n") || strings.Contains(unit, "User=") {
		t.Fatalf("unexpected Portable user unit:\n%s", unit)
	}
	for _, value := range []string{"relative.json", filepath.Join(t.TempDir(), "a\n.json")} {
		if _, err := unitArgument(value); err == nil {
			t.Fatalf("unit argument accepted %q", value)
		}
	}
}

func TestInstalledUserUnitUsesOnlyExplicitInstalledEnrollmentAction(t *testing.T) {
	t.Parallel()
	executable := filepath.Join(t.TempDir(), "usr", "lib", "ardents", "ardents")
	enrollment := filepath.Join(t.TempDir(), "package-enrollment.json")
	unit, err := enrollmentUserUnit(executable, enrollment, "enroll-installed", "Ardents Installed Endpoint (closed alpha)")
	if err != nil {
		t.Fatal(err)
	}
	escapedExecutable, _ := unitArgument(executable)
	escapedEnrollment, _ := unitArgument(enrollment)
	if !strings.Contains(unit, "Description=Ardents Installed Endpoint (closed alpha)\n") ||
		!strings.Contains(unit, "ExecStart="+escapedExecutable+" endpoint enroll-installed "+escapedEnrollment) ||
		strings.Contains(unit, "endpoint enroll ") || strings.Contains(unit, "User=") || strings.Contains(unit, "Restart=always") {
		t.Fatalf("unexpected Installed user unit:\n%s", unit)
	}
	if _, err := enrollmentUserUnit(executable, enrollment, "run", "Ardents"); err == nil {
		t.Fatal("installed unit accepted an arbitrary command action")
	}
}

func TestEntryImportRouteRejectsIncompleteCommand(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	if err := run(t.Context(), []string{"entry", "import"}, &output); err == nil || output.Len() != 0 {
		t.Fatalf("incomplete entry command err=%v output=%q", err, output.String())
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
	if err == nil || err.Error() != "usage: ardents <accept-offline|refresh-sources|endpoint|entry|name|service-instance> arguments" {
		t.Fatalf("root usage error = %v", err)
	}
}

func TestSourcePlanRejectsRetiredH3Schema(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "retired-source-plan.json")
	if err := os.WriteFile(path, []byte(`{"schema":"ardents-h3-source-plan-v1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSourcePlan(t.TempDir(), path); err == nil {
		t.Fatal("retired H3 source plan schema was accepted")
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
