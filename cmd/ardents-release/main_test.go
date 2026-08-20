package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/releasedecision"
)

// TestOfflineImportRequiresSubcommand covers the dispatch: the
// cmd must require the offline-import subcommand and reject
// every other invocation.
func TestOfflineImportRequiresSubcommand(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	var errOutput bytes.Buffer
	if err := run(nil, &output, &errOutput); err == nil {
		t.Fatal("expected error on empty arguments")
	}
	if err := run([]string{"unknown"}, &output, &errOutput); err == nil {
		t.Fatal("expected error on unknown subcommand")
	}
}

// TestOfflineImportRequiresFlags covers the flag validation: the
// cmd must reject every invocation that omits a required flag.
func TestOfflineImportRequiresFlags(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	var errOutput bytes.Buffer
	cases := [][]string{
		{"offline-import"},
		{"offline-import", "--metadata-dir", "x"},
		{"offline-import", "--capacity-ready"},
	}
	for index, arguments := range cases {
		if err := run(arguments, &output, &errOutput); err == nil {
			t.Fatalf("case %d: expected error", index)
		}
	}
}

// TestRenderedDecisionSchemaIsStable covers the JSON shape
// contract: the offline-import output uses the bounded v1 schema
// and the documented field set.
func TestRenderedDecisionSchemaIsStable(t *testing.T) {
	t.Parallel()
	decision := synthesizedDecision()
	rendered, err := json.Marshal(renderedDecision{
		Schema:         "ardents-release-decision-v1",
		Outcome:        decision.Outcome,
		Path:           decision.Path,
		Length:         decision.Length,
		Digest:         hex.EncodeToString(decision.Digest),
		Platform:       decision.Platform,
		Architecture:   decision.Architecture,
		Environment:    decision.Environment,
		Network:        decision.Network,
		ReleaseID:      decision.ReleaseIdentity,
		ReleaseVersion: decision.ReleaseVersion,
		SourceRev:      decision.SourceRevision,
		BuildInputs:    decision.BuildInputCommitment,
		BuildID:        decision.BuildIdentity,
		DependencyID:   decision.DependencyIdentity,
		SBOMID:         decision.SBOMIdentity,
		Attestation:    decision.AttestationPolicy,
		Qualification:  decision.Qualification,
		BuildState:     decision.BuildState,
		ProtocolPhase:  decision.ProtocolPhase,
		BuildSafety:    decision.BuildSafety,
		Protocol:       decision.Protocol,
		RootVersion:    decision.RootVersion,
		Floors:         floorToJSON(decision.Floors),
		Notice:         decision.Notice,
		CustodyNotice:  decision.CustodyNotice,
	})
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(rendered, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["schema"] != "ardents-release-decision-v1" {
		t.Fatalf("schema = %v, want ardents-release-decision-v1", parsed["schema"])
	}
	if _, ok := parsed["outcome"]; !ok {
		t.Fatal("outcome field missing")
	}
	if parsed["custody_notice"] == "" {
		t.Fatal("H3 custody limitation is missing")
	}
}

func TestReadMetadataDirRejectsOversizeFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "oversize.json")
	if err := os.WriteFile(path, make([]byte, maximumMetadataFileBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readMetadataDir(dir); err == nil {
		t.Fatal("oversize metadata file was accepted")
	}
}

func TestReadMetadataDirRejectsExcessFileCount(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for index := 0; index <= maximumMetadataFiles; index++ {
		name := filepath.Join(dir, fmt.Sprintf("%02d.json", index))
		if err := os.WriteFile(name, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := readMetadataDir(dir); err == nil {
		t.Fatal("excess metadata files were accepted")
	}
}

func TestReadMetadataDirBoundsAllDirectoryEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for index := 0; index <= maximumMetadataEntries; index++ {
		name := filepath.Join(dir, fmt.Sprintf("%02d.ignored", index))
		if err := os.WriteFile(name, []byte("ignored"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := readMetadataDir(dir); err == nil {
		t.Fatal("directory entry resource bomb was accepted")
	}
}

// TestReadMetadataDirIgnoresNonJSON ensures the helper does not
// surface arbitrary files as metadata. The cmd only treats .json
// files as metadata.
func TestReadMetadataDirIgnoresNonJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ignorePath := filepath.Join(dir, "ignore.txt")
	if err := os.WriteFile(ignorePath, []byte("ignore"), 0o600); err != nil {
		t.Fatal(err)
	}
	keepPath := filepath.Join(dir, "keep.json")
	if err := os.WriteFile(keepPath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, err := readMetadataDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 metadata file, got %d", len(files))
	}
	if _, ok := files["https://release.invalid/metadata/keep.json"]; !ok {
		t.Fatal("expected keep.json to be present")
	}
}

// synthesizedDecision returns a stable releasedecision.Decision
// used by the JSON-rendering test. The fields are the public
// values, not derived from any production test fixture.
func synthesizedDecision() releasedecision.Decision {
	return releasedecision.Decision{
		Outcome: releasedecision.Outcome("release-accepted"), Path: "ardents/windows-amd64/application",
		Length: 4096, Digest: make([]byte, 32), Platform: "windows-amd64", Architecture: "amd64",
		Environment: "h3-test", Network: "ardents-h3-test-1", ReleaseIdentity: "ardents-release-0001",
		ReleaseVersion: 1, SourceRevision: "rev-0001", BuildInputCommitment: "inputs-0001",
		BuildIdentity: "build-0001", DependencyIdentity: "deps-0001", SBOMIdentity: "sbom-0001",
		AttestationPolicy: "two-builder", Qualification: "qualified", BuildState: "current", ProtocolPhase: "required",
		BuildSafety: releasedecision.Outcome("release-accepted"), Protocol: releasedecision.Outcome("release-accepted"),
		RootVersion: 1, Notice: "release is accepted by every state machine",
		CustodyNotice: "H3 project-controlled custody limitation",
	}
}
