package custody

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/alphacontrol/inspection"
	"github.com/dianabuilds/ardents-network/internal/endpoint/enrollment"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestBuildAlphaInputsRejectsChangedArtifactAndUnknownRequestFieldBeforeSecret(t *testing.T) {
	endpoint := []byte("exact-linux-endpoint-artifact")
	control := []byte("exact-linux-control-artifact")
	referenceTime := time.Unix(1_800_000_100, 0).UTC()
	request := alphaInputsTestRequest(t, endpoint, control, referenceTime)
	policy := alphaInputsTestPolicy(t, "", endpoint, control)
	config := BuildAlphaInputsConfig{Root: t.TempDir(), Request: request, Endpoint: []byte("changed-endpoint"),
		Control: control, Output: filepath.Join(t.TempDir(), "static")}
	if _, err := buildAlphaInputs(context.Background(), config, unreadSecrets{}, policy, referenceTime); !errors.Is(err, ErrInvalid) {
		t.Fatalf("changed artifact error = %v", err)
	}
	var value map[string]any
	if err := json.Unmarshal(request, &value); err != nil {
		t.Fatal(err)
	}
	value["signer"] = "caller-selected"
	config.Endpoint = endpoint
	config.Request, _ = json.Marshal(value)
	if _, err := buildAlphaInputs(context.Background(), config, unreadSecrets{}, policy, referenceTime); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown signer error = %v", err)
	}
}

func TestBuildAlphaInputsRejectsAnotherProfileAndExpiredRequestBeforeSecret(t *testing.T) {
	endpoint := []byte("exact-linux-endpoint-artifact")
	control := []byte("exact-linux-control-artifact")
	referenceTime := time.Unix(1_800_000_100, 0).UTC()
	request := alphaInputsTestRequest(t, endpoint, control, referenceTime)
	policy := alphaInputsTestPolicy(t, "", endpoint, control)
	config := BuildAlphaInputsConfig{Root: t.TempDir(), Request: request, Endpoint: endpoint,
		Control: control, Output: filepath.Join(t.TempDir(), "static")}

	var value map[string]any
	if err := json.Unmarshal(request, &value); err != nil {
		t.Fatal(err)
	}
	value["profile"] = "another-alpha-profile"
	config.Request, _ = json.Marshal(value)
	if _, err := buildAlphaInputs(context.Background(), config, unreadSecrets{}, policy, referenceTime); !errors.Is(err, ErrInvalid) {
		t.Fatalf("substitute profile error = %v", err)
	}

	config.Request = request
	invokedAfterExpiry := referenceTime.Add(25 * time.Hour)
	if _, err := buildAlphaInputs(context.Background(), config, unreadSecrets{}, policy, invokedAfterExpiry); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expired request error = %v", err)
	}
}

func TestBuildAlphaInputsRejectsAnotherEnvelopeBeforeSecret(t *testing.T) {
	root := t.TempDir()
	password := []byte("release-custody-password")
	if _, err := Initialize(context.Background(), InitializeConfig{Root: root}, &fixedSecrets{values: [][]byte{password, password}}); err != nil {
		t.Fatal(err)
	}
	endpoint := []byte("exact-linux-endpoint-artifact")
	control := []byte("exact-linux-control-artifact")
	referenceTime := time.Unix(1_800_000_100, 0).UTC()
	policy := alphaInputsTestPolicy(t, root, endpoint, control)
	policy.EnvelopeSHA256 = strings.Repeat("c", 64)
	_, err := buildAlphaInputs(context.Background(), BuildAlphaInputsConfig{Root: root,
		Request: alphaInputsTestRequest(t, endpoint, control, referenceTime), Endpoint: endpoint, Control: control,
		Output: filepath.Join(t.TempDir(), "static")}, unreadSecrets{}, policy, referenceTime)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("substitute envelope error = %v", err)
	}
}

func TestBuildAlphaInputsIsDeterministicAndNeverOverwrites(t *testing.T) {
	root := t.TempDir()
	password := []byte("release-custody-password")
	if _, err := Initialize(context.Background(), InitializeConfig{Root: root}, &fixedSecrets{values: [][]byte{password, password}}); err != nil {
		t.Fatal(err)
	}
	endpoint := []byte("exact-linux-endpoint-artifact")
	control := []byte("exact-linux-control-artifact")
	request := alphaInputsTestRequest(t, endpoint, control, time.Unix(1_800_000_100, 0).UTC())
	policy := alphaInputsTestPolicy(t, root, endpoint, control)
	parent := t.TempDir()
	firstOutput, secondOutput := filepath.Join(parent, "first"), filepath.Join(parent, "second")
	first, err := buildAlphaInputs(context.Background(), BuildAlphaInputsConfig{Root: root, Request: request,
		Endpoint: endpoint, Control: control, Output: firstOutput}, &fixedSecrets{values: [][]byte{password}}, policy,
		time.Unix(1_800_000_100, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildAlphaInputs(context.Background(), BuildAlphaInputsConfig{Root: root, Request: request,
		Endpoint: endpoint, Control: control, Output: secondOutput}, &fixedSecrets{values: [][]byte{password}}, policy,
		time.Unix(1_800_000_100, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if first.OutputDigest != second.OutputDigest || len(first.Files) != len(second.Files) {
		t.Fatalf("output receipts differ: %+v / %+v", first, second)
	}
	for _, name := range alphaInputFileNames {
		left, leftErr := os.ReadFile(filepath.Join(firstOutput, name))
		right, rightErr := os.ReadFile(filepath.Join(secondOutput, name))
		if leftErr != nil || rightErr != nil || string(left) != string(right) {
			t.Fatalf("deterministic file %s: %v / %v", name, leftErr, rightErr)
		}
	}
	if _, err := buildAlphaInputs(context.Background(), BuildAlphaInputsConfig{Root: root, Request: request,
		Endpoint: endpoint, Control: control, Output: firstOutput}, unreadSecrets{}, policy,
		time.Unix(1_800_000_100, 0).UTC()); !errors.Is(err, ErrOutputExists) {
		t.Fatalf("existing output error = %v", err)
	}
}

func TestBuildAlphaInputsLeavesNoOutputWhenNetworkPreflightRejects(t *testing.T) {
	root := t.TempDir()
	password := []byte("release-custody-password")
	if _, err := Initialize(context.Background(), InitializeConfig{Root: root}, &fixedSecrets{values: [][]byte{password, password}}); err != nil {
		t.Fatal(err)
	}
	endpoint := []byte("exact-linux-endpoint-artifact")
	control := []byte("exact-linux-control-artifact")
	request := alphaInputsTestRequest(t, endpoint, control, time.Unix(1_800_000_100, 0).UTC())
	policy := alphaInputsTestPolicy(t, root, endpoint, control)
	var parsed alphaInputsRequestJSON
	if err := json.Unmarshal(request, &parsed); err != nil {
		t.Fatal(err)
	}
	parsed.NetworkState.Epoch[10] ^= 0xff
	request, err := json.Marshal(parsed)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "static")
	_, err = buildAlphaInputs(context.Background(), BuildAlphaInputsConfig{Root: root, Request: request,
		Endpoint: endpoint, Control: control, Output: output}, &fixedSecrets{values: [][]byte{password}}, policy,
		time.Unix(1_800_000_100, 0).UTC())
	if !errors.Is(err, ErrPreflight) {
		t.Fatalf("network preflight error = %v", err)
	}
	if _, statErr := os.Lstat(output); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("rejected output exists: %v", statErr)
	}
}

func TestBuildAlphaInputsPublishesOneVerifierAcceptedStaticDirectory(t *testing.T) {
	root := t.TempDir()
	password := []byte("release-custody-password")
	if _, err := Initialize(context.Background(), InitializeConfig{Root: root}, &fixedSecrets{values: [][]byte{password, password}}); err != nil {
		t.Fatal(err)
	}
	endpoint := []byte("exact-linux-endpoint-artifact")
	control := []byte("exact-linux-control-artifact")
	referenceTime := time.Unix(1_800_000_100, 0).UTC()
	request := alphaInputsTestRequest(t, endpoint, control, referenceTime)
	policy := alphaInputsTestPolicy(t, root, endpoint, control)
	output := filepath.Join(t.TempDir(), "static")

	receipt, err := buildAlphaInputs(context.Background(), BuildAlphaInputsConfig{
		Root: root, Request: request, Endpoint: endpoint, Control: control, Output: output,
	}, &fixedSecrets{values: [][]byte{password}}, policy, referenceTime)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Preflight != "accepted" || len(receipt.Files) != len(alphaInputFileNames) {
		t.Fatalf("receipt = %+v", receipt)
	}
	entries, err := os.ReadDir(output)
	if err != nil {
		t.Fatal(err)
	}
	observed := make([]string, len(entries))
	for index, entry := range entries {
		if !entry.Type().IsRegular() {
			t.Fatalf("output entry %q is not a direct regular file", entry.Name())
		}
		observed[index] = entry.Name()
	}
	want := append([]string(nil), alphaInputFileNames[:]...)
	sort.Strings(want)
	if strings.Join(observed, "\n") != strings.Join(want, "\n") {
		t.Fatalf("output inventory = %v, want %v", observed, want)
	}

	bundle, manifest := alphaInputsTestBundle(t, output, endpoint, control)
	pin := sha256.Sum256(manifest)
	report, err := inspection.Inspect(context.Background(), inspection.Config{
		Root: filepath.Join(t.TempDir(), "inspection"), At: referenceTime,
		Enrollment: enrollment.Request{
			BundleRoot: bundle, ExecutablePath: filepath.Join(bundle, "ardents-linux-amd64"),
			Pin:         enrollment.Pin{Cohort: "h4-alpha-1", Release: "h4-alpha-1-rc-1", Platform: "linux-amd64", ManifestSHA256: hex.EncodeToString(pin[:])},
			Environment: "alpha", Network: "ardents-alpha-1", TargetPath: "ardents/linux-amd64/endpoint",
			Architecture: "amd64", ReferenceTime: referenceTime,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Release != "release-accepted" || report.Inspection.Catalog != alphacontrol.OutcomeAccepted {
		t.Fatalf("preflight report = %+v", report)
	}
	for _, component := range report.Inspection.Components {
		if component.Outcome != alphacontrol.OutcomeAccepted {
			t.Fatalf("component %d = %s", component.Class, component.Outcome)
		}
	}
}

func alphaInputsTestRequest(t *testing.T, endpoint, control []byte, at time.Time) []byte {
	t.Helper()
	networkID := alphaInputsTestArray(t, "488a631a444652b50d760a739c338d5f7e54bc14e92a3c3d6002eaeead4f2d3d")
	authority := ed25519.PublicKey(alphaInputsTestHex(t, "c2f38d34dafe402561da5a0a278e8a3255e0fc9c2e58c0209966a589fd07b631"))
	epoch := alphaInputsTestFile(t, "../../network/state/testdata/epoch.hex")
	inputs := make([][]byte, 8)
	for index := range inputs {
		inputs[index] = alphaInputsTestFile(t, fmt.Sprintf("../../network/state/testdata/input-%04d.hex", index))
	}
	materials := [][]byte{alphaInputsTestFile(t, "../../network/state/testdata/materialization-0000.hex")}
	authorityID := sha256.Sum256(authority)
	opened, err := state.Open(state.Config{Root: t.TempDir(), NetworkID: networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{authorityID: authority}, Threshold: 1,
		AcceptedProfile: "h3-role-probe-v1", Now: at})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := opened.Accept(context.Background(), epoch, inputs, materials)
	if closeErr := opened.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	endpointDigest, controlDigest := sha256.Sum256(endpoint), sha256.Sum256(control)
	request := map[string]any{
		"schema": "ardents-alpha-input-request-v1", "profile": "ardents-h4-alpha-1-v1",
		"cohort": "h4-alpha-1", "release": "h4-alpha-1-rc-1", "release_version": 1,
		"reference_time": at, "not_before": at.Add(-time.Minute), "not_after": at.Add(24 * time.Hour),
		"build_safety_no_new_work_after": at.Add(12 * time.Hour), "build_safety_terminate_after": at.Add(48 * time.Hour),
		"environment": "alpha", "network": "ardents-alpha-1", "source_revision": strings.Repeat("a", 40),
		"endpoint_sha256": hex.EncodeToString(endpointDigest[:]), "control_sha256": hex.EncodeToString(controlDigest[:]),
		"build_input_commitment": "inputs-h4-alpha-1", "build_identity": "build-h4-alpha-1", "dependency_identity": "go-mod-h4-alpha-1",
		"sbom_identity": "sbom-h4-alpha-1", "qualification": "qualified", "build_state": "current", "protocol_phase": "announced",
		"builders": []string{"windows-custody-workstation", "ubuntu-project-builder"},
		"network_state": map[string]any{"network_id": hex.EncodeToString(networkID[:]), "epoch_digest": hex.EncodeToString(snapshot.Digest[:]),
			"profile": "h3-role-probe-v1", "threshold": 1, "authorities": []string{hex.EncodeToString(authority)},
			"epoch": epoch, "inputs": inputs, "materials": materials},
	}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func alphaInputsTestPolicy(t *testing.T, root string, endpoint, control []byte) alphaInputPolicy {
	t.Helper()
	endpointDigest, controlDigest := sha256.Sum256(endpoint), sha256.Sum256(control)
	policy := alphaInputPolicy{Profile: "ardents-h4-alpha-1-v1", SourceRevision: strings.Repeat("a", 40),
		EndpointSHA256: hex.EncodeToString(endpointDigest[:]), ControlSHA256: hex.EncodeToString(controlDigest[:]),
		EnvelopeSHA256: strings.Repeat("b", 64)}
	if root != "" {
		raw, err := os.ReadFile(seedPath(root))
		if err != nil {
			t.Fatal(err)
		}
		envelopeDigest := sha256.Sum256(raw)
		policy.EnvelopeSHA256 = hex.EncodeToString(envelopeDigest[:])
	}
	return policy
}

func alphaInputsTestBundle(t *testing.T, static string, endpoint, control []byte) (string, []byte) {
	t.Helper()
	bundle := filepath.Join(t.TempDir(), "bundle")
	if err := os.Mkdir(bundle, 0o700); err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte, len(alphaInputFileNames)+2)
	for _, name := range alphaInputFileNames {
		contents, err := os.ReadFile(filepath.Join(static, name))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = contents
	}
	files["ardents-linux-amd64"], files["ardents-control-linux-amd64"] = endpoint, control
	names := make([]string, 0, len(files))
	for name, contents := range files {
		mode := os.FileMode(0o600)
		if strings.HasPrefix(name, "ardents-") {
			mode = 0o700
		}
		if err := os.WriteFile(filepath.Join(bundle, name), contents, mode); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	var manifest strings.Builder
	for _, name := range names {
		value := sha256.Sum256(files[name])
		fmt.Fprintf(&manifest, "%x  %s\n", value, name)
	}
	raw := []byte(manifest.String())
	if err := os.WriteFile(filepath.Join(bundle, "SHA256SUMS"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return bundle, raw
}

func alphaInputsTestFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func alphaInputsTestHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func alphaInputsTestArray(t *testing.T, value string) [32]byte {
	t.Helper()
	var result [32]byte
	copy(result[:], alphaInputsTestHex(t, value))
	return result
}
