package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/release"
	"github.com/dianabuilds/ardents-network/internal/updatetransaction"
)

func TestHiddenApplyErrorRetainsCauseWithoutRenderingIt(t *testing.T) {
	cause := errors.New(`C:\private\update-root`)
	wrapped := fmt.Errorf("apply failed: %w", hiddenApplyError{cause})
	if !errors.Is(wrapped, cause) {
		t.Fatal("bounded error lost its underlying cause")
	}
	if strings.Contains(wrapped.Error(), "private") {
		t.Fatalf("bounded error rendered private cause: %q", wrapped)
	}
}

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
	rendered, err := renderDecision(decision)
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
	want := "{\"schema\":\"ardents-release-decision-v1\",\"outcome\":\"release-accepted\",\"path\":\"ardents/windows-amd64/application\",\"length\":4096," +
		"\"digest\":\"0000000000000000000000000000000000000000000000000000000000000000\",\"platform\":\"windows-amd64\",\"architecture\":\"amd64\",\"environment\":\"h3-test\",\"network\":\"ardents-h3-test-1\"," +
		"\"release_identity\":\"ardents-release-0001\",\"release_version\":1,\"source_revision\":\"rev-0001\",\"build_input_commitment\":\"inputs-0001\",\"build_identity\":\"build-0001\",\"dependency_identity\":\"deps-0001\"," +
		"\"sbom_identity\":\"sbom-0001\",\"attestation_policy\":\"two-builder\",\"qualification\":\"qualified\",\"build_state\":\"current\",\"protocol_phase\":\"required\",\"build_safety\":\"release-accepted\",\"protocol\":\"release-accepted\",\"root_version\":1," +
		"\"floors\":{\"root_version\":0,\"root_digest\":\"\",\"timestamp_version\":0,\"timestamp_digest\":\"\",\"snapshot_version\":0,\"snapshot_digest\":\"\",\"targets_version\":0,\"targets_digest\":\"\"}," +
		"\"notice\":\"release is accepted by every state machine\",\"custody_notice\":\"H3 project-controlled custody limitation\"}\n"
	if string(rendered) != want {
		t.Fatalf("offline-import JSON changed: %q", rendered)
	}
}

func TestRenderedUpdateResultIsExact(t *testing.T) {
	t.Parallel()
	current := [32]byte{0xa5, 0x2b}
	rollback := [32]byte{0x8b, 0xde}
	rendered, err := renderUpdateResult(updatetransaction.Result{
		Outcome: "committed", State: "committed", Generation: 1,
		CurrentDigest: current, RollbackDigest: rollback,
		SafeNotice: "update committed", CustodyNotice: "bounded custody notice",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"schema\":\"ardents-update-result-v1\",\"outcome\":\"committed\",\"state\":\"committed\"," +
		"\"transaction_generation\":1,\"current_sha256\":\"a52b000000000000000000000000000000000000000000000000000000000000\"," +
		"\"rollback_sha256\":\"8bde000000000000000000000000000000000000000000000000000000000000\"," +
		"\"staging_present\":false,\"safe_notice\":\"update committed\",\"custody_notice\":\"bounded custody notice\"}\n"
	if string(rendered) != want {
		t.Fatalf("rendered result = %q, want %q", rendered, want)
	}
}

func TestApplyOfflineV0IsExact(t *testing.T) {
	parent := t.TempDir()
	updateRoot := filepath.Join(parent, "update")
	bootstrapUpdateRoot(t, updateRoot)
	releaseRoot := filepath.Join(parent, "release")
	fixture := filepath.Join("..", "..", "internal", "release", "testdata", "r049-public-vector-v1")
	metadata := filepath.Join(parent, "metadata")
	if err := os.Mkdir(metadata, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"timestamp.json", "1.snapshot.json", "1.targets.json"} {
		value, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		writeFixture(t, filepath.Join(metadata, name), value)
	}
	arguments := []string{"apply-offline", "-state-root", releaseRoot,
		"-metadata-dir", metadata, "-root", filepath.Join(fixture, "root.json"),
		"-target", "ardents/windows-amd64/application", "-artifact", filepath.Join(fixture, "artifact.bin"),
		"-environment", "h3-test", "-network", "ardents-h3-test-1", "-platform", "windows-amd64",
		"-architecture", "amd64", "-ref-time", "2030-01-02T03:04:05Z", "-update-root", updateRoot}
	var output, errorOutput bytes.Buffer
	if err := run(arguments, &output, &errorOutput); err != nil {
		t.Fatalf("apply-offline: %v; stderr=%q", err, errorOutput.String())
	}
	want := "{\"schema\":\"ardents-update-result-v1\",\"outcome\":\"committed\",\"state\":\"committed\"," +
		"\"transaction_generation\":1,\"current_sha256\":\"a52b68413e0cd723547790c7ac161ece935d6459377442644b18031c3dc27d0a\"," +
		"\"rollback_sha256\":\"8bdad9bde29bb6ee2a9d1d7005ec8ba2461b2bad3627372ee8458693c1fc08af\"," +
		"\"staging_present\":false,\"safe_notice\":\"update committed\"," +
		"\"custody_notice\":\"H3 threshold identities and both rebuild records are project-controlled; no independent custody or builder claim is made\"}\n"
	if output.String() != want {
		t.Fatalf("apply-offline output = %q, want %q", output.String(), want)
	}
}

func bootstrapUpdateRoot(t *testing.T, root string) {
	t.Helper()
	previous, err := os.ReadFile(filepath.Join("..", "..", "docs", "development", "testdata", "s7.2", "previous-payload-v1.txt"))
	if err != nil {
		t.Fatal(err)
	}
	previousDigest := sha256.Sum256(previous)
	body := appendUint64(nil, 0)
	body = appendText(body, "ardents/windows-amd64/application")
	body = appendUint64(body, uint64(len(previous)))
	body = append(body, previousDigest[:]...)
	for _, value := range []string{"windows-amd64", "amd64", "h3-test", "ardents-h3-test-1", "previous-v1"} {
		body = appendText(body, value)
	}
	body = appendUint64(body, 0)
	for range 9 {
		body = appendText(body, "bootstrap-preserved")
	}
	body = appendText(appendText(body, "release-accepted"), "release-accepted")
	for _, floor := range []struct {
		version uint64
		digest  string
	}{{1, "246c88b483ccb15982710fa661f7e456f9361f95c2529df9d60082c5c35c59fd"},
		{1, "12fcf1537e5f8ea10a3cbb69b95bfd2af43ce1964d84719f668a55d3d06158cb"},
		{1, "5adca7af592d9d686e6b1033c2ef0c8cf60348e80942e4ad98c97a1008d67c02"},
		{1, "2bbcb3498c44715a5d36bdc7c91b1fba4c3d639e385db5554b7b04f2b404ae73"}} {
		body = appendUint64(body, floor.version)
		body = append(body, decodeHex(t, floor.digest)...)
	}
	for _, value := range []string{"2030-01-02T03:04:05Z", "2030-02-01T03:04:05Z", "2030-07-01T03:04:05Z"} {
		parsed, parseErr := time.Parse(time.RFC3339, value)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		body = appendUint64(body, uint64(parsed.Unix()))
	}
	body = appendUint64(body, 0)
	body = appendText(appendText(appendText(body, "no-op-v1"), "bootstrap generation preserved"),
		"H3 threshold identities and both rebuild records are project-controlled; no independent custody or builder claim is made")
	for _, value := range []string{"release-accepted", "windows-amd64", "amd64", "h3-test", "ardents-h3-test-1"} {
		body = appendText(body, value)
	}
	body = append(body, 1, 0, 1)
	manifest := envelope(1, body)
	manifestDigest := sha256.Sum256(manifest)
	current := appendUint64(nil, 0)
	current = appendUint64(current, 0)
	current = appendUint64(current, uint64(len(previous)))
	current = append(current, previousDigest[:]...)
	current = append(current, manifestDigest[:]...)
	current = append(current, 0)
	for _, directory := range []string{filepath.Join(root, "generations", "0"), filepath.Join(root, "staging"), filepath.Join(root, "transactions")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeFixture(t, filepath.Join(root, ".ardents-update-transaction-lock"), nil)
	writeFixture(t, filepath.Join(root, ".ardents-update-transaction-v1"), []byte("ardents-update-transaction-v1\n"))
	writeFixture(t, filepath.Join(root, "current"), envelope(2, current))
	writeFixture(t, filepath.Join(root, "generations", "0", "artifact"), previous)
	writeFixture(t, filepath.Join(root, "generations", "0", "manifest.bin"), manifest)
}

func appendUint64(body []byte, value uint64) []byte {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	return append(body, encoded[:]...)
}

func appendText(body []byte, value string) []byte {
	var length [2]byte
	binary.BigEndian.PutUint16(length[:], uint16(len(value)))
	return append(append(body, length[:]...), value...)
}

func envelope(kind byte, body []byte) []byte {
	raw := append([]byte("ARDUPD01"), kind, 1, 0, 0)
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(body)))
	return append(append(raw, length[:]...), body...)
}

func decodeHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func writeFixture(t *testing.T, path string, value []byte) {
	t.Helper()
	if err := os.WriteFile(path, value, 0o600); err != nil {
		t.Fatal(err)
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

// synthesizedDecision returns a stable release.Decision
// used by the JSON-rendering test. The fields are the public
// values, not derived from any production test fixture.
func synthesizedDecision() release.Decision {
	return release.Decision{
		Outcome: release.Outcome("release-accepted"), Path: "ardents/windows-amd64/application",
		Length: 4096, Digest: make([]byte, 32), Platform: "windows-amd64", Architecture: "amd64",
		Environment: "h3-test", Network: "ardents-h3-test-1", ReleaseIdentity: "ardents-release-0001",
		ReleaseVersion: 1, SourceRevision: "rev-0001", BuildInputCommitment: "inputs-0001",
		BuildIdentity: "build-0001", DependencyIdentity: "deps-0001", SBOMIdentity: "sbom-0001",
		AttestationPolicy: "two-builder", Qualification: "qualified", BuildState: "current", ProtocolPhase: "required",
		BuildSafety: release.Outcome("release-accepted"), Protocol: release.Outcome("release-accepted"),
		RootVersion: 1, Notice: "release is accepted by every state machine",
		CustodyNotice: "H3 project-controlled custody limitation",
	}
}
