//go:build linux && network_signing_native

package signing

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/release/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func sourceNativePinnedProgram(t *testing.T, variable string) (string, []byte) {
	t.Helper()
	path, pin := os.Getenv(variable), os.Getenv(variable+"_SHA256")
	raw, err := os.ReadFile(path)
	digest := sha256.Sum256(raw)
	if err != nil || !filepath.IsAbs(path) || len(pin) != 64 || hex.EncodeToString(digest[:]) != pin {
		t.Fatal("invalid environment: original independently pinned program required", variable, err)
	}
	return path, raw
}

func releaseCommand(t *testing.T, program string, args ...string) *exec.Cmd {
	return exec.CommandContext(t.Context(), program, append([]string{"infrastructure", "sign-release"}, args...)...)
}

// The native test process is an actual executable of a different Go command.
func foreignNativeProgram(t *testing.T) []byte {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// Fresh real signatures and actual canonical binaries exercise this closed
// offline purpose. Fixture build/qualification declarations are controlled
// inputs, not operational rebuild evidence or installed qualification.
func runInfrastructureFixtureCommand(t *testing.T, program string, args ...string) {
	t.Helper()
	command := exec.CommandContext(t.Context(), program, append([]string{"infrastructure"}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil || !bytes.Contains(output, []byte(`"outcome":"infrastructure-public-prepared"`)) {
		t.Fatalf("public infrastructure preparation %q: %v %s", args, err, output)
	}
}

func TestSourcePreparationNativeInitialReleasePurposeAndKeyCustody(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	command, _ := sourceNativePinnedProgram(t, "ARDENTS_INFRASTRUCTURE_RELEASE_PROGRAM")
	nextPath, nextBytes := sourceNativePinnedProgram(t, "ARDENTS_SOURCE_NEXT_PROGRAM")
	preparedTemplate, err := exec.CommandContext(t.Context(), nextPath, "platform", "template").Output()
	if err != nil || !bytes.Equal(preparedTemplate, []byte(systemd.PlatformUnitTemplate)) {
		t.Fatal("canonical platform template producer", err)
	}
	program, template, descriptor := filepath.Join(parent, "node"), filepath.Join(parent, "template"), filepath.Join(parent, "descriptor")
	for path, raw := range map[string][]byte{program: nextBytes, template: preparedTemplate} {
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	descriptorBytes, err := infrastructure.PrepareRuntimeDescriptor("source", "one", 1, nextBytes, []byte(systemd.PlatformUnitTemplate))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(descriptor, descriptorBytes, 0600); err != nil {
		t.Fatal(err)
	}
	keyRoot := filepath.Join(parent, "keys")
	keyOutput, err := releaseCommand(t, command, "source", "prepare-release-keys", keyRoot).CombinedOutput()
	if err != nil {
		t.Fatal("actual Source Release key birth", string(keyOutput), err)
	}
	var keyReceipt struct {
		Operation   string `json:"operation"`
		Threshold   int    `json:"ordinary_threshold"`
		Independent bool   `json:"independent_custody"`
		Keys        []struct {
			File   string `json:"file"`
			Public string `json:"ed25519_public_hex"`
			Digest string `json:"public_sha256"`
		} `json:"keys"`
	}
	if json.Unmarshal(keyOutput, &keyReceipt) != nil || keyReceipt.Operation != "source.prepare-release-keys" || keyReceipt.Threshold != 3 || keyReceipt.Independent || len(keyReceipt.Keys) != 5 {
		t.Fatal("public key receipt", string(keyOutput))
	}
	seenKeys := map[string]bool{}
	for i, entry := range keyReceipt.Keys {
		if entry.File != fmt.Sprintf("release-key-%d.pem", i+1) || seenKeys[entry.Public] {
			t.Fatal("fixed distinct key inventory")
		}
		seenKeys[entry.Public] = true
		raw, err := os.ReadFile(filepath.Join(keyRoot, entry.File))
		if err != nil {
			t.Fatal(err)
		}
		block, rest := pem.Decode(raw)
		clear(raw)
		if block == nil || block.Type != "PRIVATE KEY" || len(rest) != 0 {
			t.Fatal("private encoding")
		}
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		clear(block.Bytes)
		if err != nil {
			t.Fatal(err)
		}
		private, ok := key.(ed25519.PrivateKey)
		if !ok {
			t.Fatal("Ed25519 required")
		}
		public := private.Public().(ed25519.PublicKey)
		clear(private)
		digest := sha256.Sum256(public)
		if hex.EncodeToString(public) != entry.Public || hex.EncodeToString(digest[:]) != entry.Digest {
			t.Fatal("independent private/public receipt mismatch")
		}
	}
	now := time.Now().UTC().Truncate(time.Second)
	custom := func(artifact []byte) json.RawMessage {
		digest := sha256.Sum256(artifact)
		attestations := []map[string]any{}
		for _, builder := range []string{"controlled-fixture-one", "controlled-fixture-two"} {
			attestations = append(attestations, map[string]any{"builder_identity": builder, "build_identity": "fixture-build", "source_revision": "fixture-source", "build_input_commitment": "fixture-input", "target_sha256": hex.EncodeToString(digest[:])})
		}
		raw, err := json.Marshal(map[string]any{"schema_version": 1, "profile": "ardents-h3-release-v1", "platform": "linux-amd64", "architecture": "amd64", "environment": "alpha", "network": "one-network", "release_identity": "one", "release_version": 1,
			"source_revision": "fixture-source", "build_input_commitment": "fixture-input", "build_identity": "fixture-build", "dependency_identity": "fixture-dependencies", "sbom_identity": "fixture-sbom", "attestation_policy": "two-builder", "qualification": "qualified", "build_state": "current", "builder_attestations": attestations,
			"build_safety_no_new_work_after": now.Add(2 * time.Hour), "build_safety_terminate_after": now.Add(3 * time.Hour), "protocol_phase": "announced", "protocol_overlapped_since": time.Time{}, "capacity_ready": false, "drain_ready": false, "emergency_reason": "", "emergency_expiry": time.Time{}})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	plan := sourceReleasePlan{Schema: "ardents-source-initial-release-plan-v1", Network: "one-network", Reference: now, Expires: now.Add(time.Hour), Program: program, Descriptor: descriptor, Template: template, ProgramCustom: custom(nextBytes), GenerationCustom: custom(descriptorBytes)}
	planPath := filepath.Join(parent, "plan")
	writePlan := func() {
		raw, err := json.Marshal(plan)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(planPath, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	writePlan()
	output, verification := filepath.Join(parent, "metadata"), filepath.Join(parent, "verification")
	child := releaseCommand(t, command, "source", "sign-initial-release", planPath, keyRoot, output, verification)
	if out, err := child.CombinedOutput(); err != nil || !bytes.Contains(out, []byte(`"operation":"source.sign-initial-release"`)) {
		t.Fatal("actual canonical Source signature operation", string(out), err)
	}
	rootBytes, err := os.ReadFile(filepath.Join(output, "root.json"))
	if err != nil {
		t.Fatal(err)
	}
	root, err := metadata.Root().FromBytes(rootBytes)
	if err != nil || len(root.Signed.Keys) != 5 || len(root.Signatures) != 3 {
		t.Fatal("ordinary five-key/three-signature Root", err)
	}
	for _, role := range metadata.TOP_LEVEL_ROLE_NAMES {
		if root.Signed.Roles[role].Threshold != 3 || len(root.Signed.Roles[role].KeyIDs) != 5 {
			t.Fatal("role threshold changed", role)
		}
	}
	targetBytes, err := os.ReadFile(filepath.Join(output, "targets.json"))
	if err != nil {
		t.Fatal(err)
	}
	targets, err := metadata.Targets().FromBytes(targetBytes)
	if err != nil || len(targets.Signed.Targets) != 2 || targets.Signed.Targets["ardents/linux-amd64/node"] == nil || targets.Signed.Targets["ardents/linux-amd64/infrastructure/source"] == nil || len(targets.Signatures) != 3 {
		t.Fatal("Source target purpose changed", err)
	}
	// Assemble the complete actual predecessor distribution without a new pin.
	// The forward producer must keep its Root and genuinely advance the same
	// diagnostic Release history; its output is no installation permission.
	previous := filepath.Join(parent, "previous-distribution")
	if err := os.Mkdir(previous, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		raw, err := os.ReadFile(filepath.Join(output, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(previous, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for name, raw := range map[string][]byte{infrastructure.ProgramName: nextBytes, infrastructure.TemplateName: preparedTemplate, infrastructure.DescriptorName: descriptorBytes} {
		if err := os.WriteFile(filepath.Join(previous, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	inventoryOutput := filepath.Join(parent, "previous-inventory")
	runInfrastructureFixtureCommand(t, nextPath, "prepare-runtime-inventory", "source", previous, inventoryOutput)
	inventoryRaw, err := os.ReadFile(filepath.Join(inventoryOutput, "infrastructure-enrollment.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(previous, "infrastructure-enrollment.json"), inventoryRaw, 0600); err != nil {
		t.Fatal(err)
	}
	exerciseSourceTimestampRefresh(t, sourceTimestampNativeInput{command: command, nextProgram: nextPath, planPath: planPath, keys: keyRoot,
		parent: parent, distribution: previous, plan: plan, program: nextBytes, descriptor: descriptorBytes})
	forwardDescriptor, err := infrastructure.PrepareRuntimeDescriptor("source", "two", 2, nextBytes, preparedTemplate)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(descriptor, forwardDescriptor, 0600); err != nil {
		t.Fatal(err)
	}
	forwardCustom := func(artifact []byte) json.RawMessage {
		var fields map[string]any
		if err := json.Unmarshal(custom(artifact), &fields); err != nil {
			t.Fatal(err)
		}
		fields["release_identity"], fields["release_version"] = "two", 2
		raw, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	initialPlan := plan
	plan.Schema = "ardents-source-forward-release-plan-v1"
	plan.ProgramCustom, plan.GenerationCustom = forwardCustom(nextBytes), forwardCustom(forwardDescriptor)
	writePlan()
	forwardOutput, forwardHistory := filepath.Join(parent, "forward-metadata"), filepath.Join(parent, "forward-verification")
	forwardArgs := []string{planPath, keyRoot, forwardOutput, forwardHistory, previous}
	forwardChild := releaseCommand(t, command, append([]string{"source", "sign-forward-release"}, forwardArgs...)...)
	if out, err := forwardChild.CombinedOutput(); err != nil || !bytes.Contains(out, []byte(`"operation":"source.sign-forward-release"`)) {
		t.Fatal("canonical forward signing", string(out), err)
	}
	forwardRoot, err := os.ReadFile(filepath.Join(forwardOutput, "root.json"))
	if err != nil || !bytes.Equal(forwardRoot, rootBytes) {
		t.Fatal("forward Root changed", err)
	}
	for _, name := range []string{"timestamp.json", "snapshot.json", "targets.json"} {
		raw, err := os.ReadFile(filepath.Join(forwardOutput, name))
		var value struct {
			Signed struct {
				Version int64 `json:"version"`
			} `json:"signed"`
		}
		if err != nil || json.Unmarshal(raw, &value) != nil || value.Signed.Version != 2 {
			t.Fatal("metadata did not advance", name, err)
		}
	}
	// A genuinely valid forward generation is not a timestamp-only request.
	plan.Schema = "ardents-source-timestamp-release-plan-v1"
	writePlan()
	timestampOutput, timestampHistory := filepath.Join(parent, "changed-timestamp-metadata"), filepath.Join(parent, "changed-timestamp-verification")
	if raw, err := signSourceRelease(t.Context(), []string{planPath, keyRoot, timestampOutput, timestampHistory, previous}, sourceReleaseTimestamp, nil); err == nil || raw != nil {
		t.Fatal("timestamp purpose changed generation", err)
	}
	for _, path := range []string{timestampOutput, timestampHistory} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("changed-generation timestamp dependent effect", path, err)
		}
	}
	plan.Schema = "ardents-source-forward-release-plan-v1"
	// Equal descriptor version refuses before any output/diagnostic birth.
	if err := os.WriteFile(descriptor, descriptorBytes, 0600); err != nil {
		t.Fatal(err)
	}
	plan.ProgramCustom, plan.GenerationCustom = custom(nextBytes), custom(descriptorBytes)
	writePlan()
	refusedOutput, refusedHistory := filepath.Join(parent, "equal-metadata"), filepath.Join(parent, "equal-verification")
	if raw, err := signSourceRelease(t.Context(), []string{planPath, keyRoot, refusedOutput, refusedHistory, previous}, sourceReleaseForward, nil); err == nil || raw != nil {
		t.Fatal("equal release signed", err)
	}
	for _, path := range []string{refusedOutput, refusedHistory} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("equal-version dependent effect", path, err)
		}
	}
	plan = initialPlan
	writePlan()
	// A real Control ELF cannot be substituted for the Source target, even with
	// matching descriptor hashes and otherwise accepted fixture declarations.
	foreignProgram := foreignNativeProgram(t)
	if err := os.WriteFile(program, foreignProgram, 0600); err != nil {
		t.Fatal(err)
	}
	badDescriptor, err := infrastructure.PrepareRuntimeDescriptor("source", "one", 1, foreignProgram, []byte(systemd.PlatformUnitTemplate))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(descriptor, badDescriptor, 0600); err != nil {
		t.Fatal(err)
	}
	plan.ProgramCustom, plan.GenerationCustom = custom(foreignProgram), custom(badDescriptor)
	writePlan()
	signed := false
	if raw, err := signSourceRelease(t.Context(), []string{planPath, keyRoot, filepath.Join(parent, "wrong-program"), filepath.Join(parent, "wrong-verification")}, sourceReleaseInitial, func(string, *os.File) { signed = true }); err == nil || raw != nil || signed {
		t.Fatal("wrong executable reached signing", err, signed)
	}
	if err := os.WriteFile(program, nextBytes, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(descriptor, descriptorBytes, 0600); err != nil {
		t.Fatal(err)
	}
	plan.ProgramCustom, plan.GenerationCustom = custom(nextBytes), custom(descriptorBytes)
	writePlan()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	seen := false
	raw, err := signSourceRelease(ctx, []string{planPath, keyRoot, filepath.Join(parent, "failed-metadata"), filepath.Join(parent, "failed-verification")}, sourceReleaseInitial, func(stage string, file *os.File) {
		if stage != "signed" {
			return
		}
		seen = true
		cancel()
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	if !seen || raw != nil || !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) {
		t.Fatal("original signing/cancellation/physical custody loss published", seen, err)
	}
}
