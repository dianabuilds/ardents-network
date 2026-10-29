//go:build linux && network_signing_native

package main

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

	"github.com/dianabuilds/ardents-network/internal/successor/installation/forwarding"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestForwardingPreparationNativeInitialReleasePurposeAndKeyCustody(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	command, controlBytes := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_CONTROL_PROGRAM")
	_, nodeBytes := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_NODE_PROGRAM")
	templateOutput := filepath.Join(parent, "template-output")
	if out, err := exec.CommandContext(t.Context(), command, "forwarding", "prepare-template", templateOutput).CombinedOutput(); err != nil {
		t.Fatal("actual fixed template producer", string(out), err)
	}
	preparedTemplate, err := os.ReadFile(filepath.Join(templateOutput, "ardents-forwarding.service"))
	if err != nil || !bytes.Equal(preparedTemplate, []byte(forwarding.Template)) {
		t.Fatal("closed template differs", err)
	}
	program, template, descriptor := filepath.Join(parent, "node"), filepath.Join(parent, "template"), filepath.Join(parent, "descriptor")
	for path, raw := range map[string][]byte{program: nodeBytes, template: preparedTemplate} {
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	descriptorBytes, err := infrastructure.PrepareForwardingDescriptor("one", 1, nodeBytes, []byte(forwarding.Template))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(descriptor, descriptorBytes, 0600); err != nil {
		t.Fatal(err)
	}
	keyRoot := filepath.Join(parent, "keys")
	keyOutput, err := exec.CommandContext(t.Context(), command, "forwarding", "prepare-release-keys", keyRoot).CombinedOutput()
	if err != nil {
		t.Fatal("actual Forwarding Release key birth", string(keyOutput), err)
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
	if json.Unmarshal(keyOutput, &keyReceipt) != nil || keyReceipt.Operation != "forwarding.prepare-release-keys" || keyReceipt.Threshold != 3 || keyReceipt.Independent || len(keyReceipt.Keys) != 5 {
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
	if _, err := prepareForwardingReleaseKeys(t.Context(), keyRoot, nil); err == nil {
		t.Fatal("Release key prefix adopted")
	}
	keyCtx, keyCancel := context.WithCancel(t.Context())
	defer keyCancel()
	failedPublic, keyErr := prepareForwardingReleaseKeys(keyCtx, filepath.Join(parent, "failed-keys"), func(stage string, file *os.File) {
		if stage == "before-directory" {
			keyCancel()
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
		}
	})
	if failedPublic != ([5][32]byte{}) || !errors.Is(keyErr, context.Canceled) || !errors.Is(keyErr, os.ErrClosed) {
		t.Fatal("failed key custody published public success/lost physical failure", keyErr)
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
	plan := forwardingReleasePlan{Schema: "ardents-forwarding-initial-release-plan-v1", Network: "one-network", Reference: now, Expires: now.Add(time.Hour), Program: program, Descriptor: descriptor, Template: template, ProgramCustom: custom(nodeBytes), GenerationCustom: custom(descriptorBytes)}
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
	// Legitimate workflow uses the public flag-based canonical plan producer.
	programClaims, generationClaims := filepath.Join(parent, "program-claims.json"), filepath.Join(parent, "generation-claims.json")
	if err := os.WriteFile(programClaims, plan.ProgramCustom, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(generationClaims, plan.GenerationCustom, 0600); err != nil {
		t.Fatal(err)
	}
	planOutput := filepath.Join(parent, "prepared-release-plan")
	publicArgs := []string{"forwarding", "prepare-release-plan", "--network", plan.Network, "--reference", plan.Reference.Format(time.RFC3339), "--expires", plan.Expires.Format(time.RFC3339), "--program", program, "--descriptor", descriptor, "--template", template, "--program-custom", programClaims, "--generation-custom", generationClaims, "--output", planOutput}
	if out, err := exec.CommandContext(t.Context(), command, publicArgs...).CombinedOutput(); err != nil {
		t.Fatal("canonical public release plan producer", err, string(out))
	}
	planPath = filepath.Join(planOutput, "forwarding-release-plan.json")
	output, verification := filepath.Join(parent, "metadata"), filepath.Join(parent, "verification")
	child := exec.CommandContext(t.Context(), command, "forwarding", "sign-initial-release", planPath, keyRoot, output, verification)
	if out, err := child.CombinedOutput(); err != nil || !bytes.Contains(out, []byte(`"operation":"forwarding.sign-initial-release"`)) {
		t.Fatal("actual canonical Forwarding signature operation", string(out), err)
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
	if err != nil || len(targets.Signed.Targets) != 2 || targets.Signed.Targets["ardents/linux-amd64/node"] == nil || targets.Signed.Targets["ardents/linux-amd64/infrastructure/forwarding"] == nil || len(targets.Signatures) != 3 {
		t.Fatal("Forwarding target purpose changed", err)
	}

	// These are causal failures of original native custody. Neither a completed
	// signature nor complete-looking metadata may erase cancellation or Close.
	for _, cut := range []string{"signed", "before-file", "commit"} {
		t.Run("original-custody-"+cut, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			metadataPath := filepath.Join(parent, "failed-"+cut)
			verificationPath := metadataPath + "-verification"
			fired := false
			root, err := signForwardingReleasePurpose(ctx, []string{planPath, keyRoot, metadataPath, verificationPath}, forwardingReleaseInitial, func(stage string, file *os.File) {
				if stage != cut || fired {
					return
				}
				fired = true
				cancel()
				if err := file.Close(); err != nil {
					t.Fatal("original descriptor close", err)
				}
			})
			if !fired || len(root) != 0 || !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) {
				t.Fatal("failed native custody published success or lost cause", fired, err)
			}
			if cut == "signed" {
				if _, err := os.Lstat(metadataPath); !os.IsNotExist(err) {
					t.Fatal("lost key custody reached metadata output", err)
				}
				return
			}
			before, err := os.Stat(metadataPath)
			if err != nil {
				t.Fatal("partial output provenance lost", err)
			}
			if root, err := signForwardingReleasePurpose(t.Context(), []string{planPath, keyRoot, metadataPath, verificationPath}, forwardingReleaseInitial, nil); err == nil || len(root) != 0 {
				t.Fatal("partial output adopted as fresh signing")
			}
			after, err := os.Stat(metadataPath)
			if err != nil || !os.SameFile(before, after) {
				t.Fatal("retained original partial output replaced", err)
			}
		})
	}

	// Neither another declared purpose nor an arbitrary executable may reach signing.
	for _, bad := range []string{"ardents-source-initial-release-plan-v1", "ardents-forwarding-forward-release-plan-v1"} {
		plan.Schema = bad
		writePlan()
		output := filepath.Join(parent, bad)
		if _, err := signForwardingReleasePurpose(t.Context(), []string{planPath, keyRoot, output, output + "-verification"}, forwardingReleaseInitial, nil); err == nil {
			t.Fatal("foreign purpose reached initial signing", bad)
		}
		if _, err := os.Lstat(output); !os.IsNotExist(err) {
			t.Fatal("foreign purpose created metadata", err)
		}
	}
	plan.Schema = "ardents-forwarding-initial-release-plan-v1"
	if err := os.WriteFile(program, controlBytes, 0600); err != nil {
		t.Fatal(err)
	}
	badDescriptor, err := infrastructure.PrepareForwardingDescriptor("one", 1, controlBytes, []byte(forwarding.Template))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(descriptor, badDescriptor, 0600); err != nil {
		t.Fatal(err)
	}
	plan.ProgramCustom, plan.GenerationCustom = custom(controlBytes), custom(badDescriptor)
	writePlan()
	if _, err := signForwardingReleasePurpose(t.Context(), []string{planPath, keyRoot, filepath.Join(parent, "bad-program"), filepath.Join(parent, "bad-program-history")}, forwardingReleaseInitial, nil); err == nil {
		t.Fatal("Control executable signed as Node")
	}
}

// Actual controlled double-build records feed the public preparer and the
// existing canonical signer. Neither the records nor signatures supply an
// independent builder, first pin, installed qualification or bootstrap right.
func TestForwardingPreparationNativeActualBuildRecordsAndPublicPlan(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	command, controlBytes := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_CONTROL_PROGRAM")
	_, nodeBytes := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_NODE_PROGRAM")
	buildRoot := os.Getenv("ARDENTS_FORWARDING_BUILD_ROOT")
	preparer, _ := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_RELEASE_PREPARER")
	evidence, _ := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_BUILD_EVIDENCE")
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	digest := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	if !filepath.IsAbs(buildRoot) || digest(read(filepath.Join(buildRoot, "READY"))) != os.Getenv("ARDENTS_FORWARDING_BUILD_READY_SHA256") {
		t.Fatal("invalid environment: actual controlled build commitment required")
	}
	for _, number := range []string{"build-1", "build-2"} {
		if !bytes.Equal(read(filepath.Join(buildRoot, number, "ardents-node")), nodeBytes) || !bytes.Equal(read(filepath.Join(buildRoot, number, "ardents-control")), controlBytes) {
			t.Fatal("actual canonical commands differ from controlled build records")
		}
	}
	call := func(args ...string) []byte {
		t.Helper()
		raw, err := exec.CommandContext(t.Context(), command, append([]string{"forwarding"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal("canonical build-bound preparation", args[0], err, string(raw))
		}
		return raw
	}
	templateRoot, descriptorRoot := filepath.Join(parent, "template"), filepath.Join(parent, "descriptor")
	call("prepare-template", templateRoot)
	template := filepath.Join(templateRoot, "ardents-forwarding.service")
	call("prepare-descriptor", "controlled-forwarding-build", "1", filepath.Join(buildRoot, "build-1", "ardents-node"), template, descriptorRoot)
	descriptor := filepath.Join(descriptorRoot, "infrastructure-forwarding.json")
	now := time.Now().UTC().Truncate(time.Second)
	arguments := func(build, declaration, output string) []string {
		return []string{"-I", "-S", preparer, "--build-root", build, "--descriptor", declaration, "--template", template, "--network", "controlled-forwarding-build", "--expires", now.Add(time.Hour).Format(time.RFC3339), "--no-new-work-after", now.Add(30 * time.Minute).Format(time.RFC3339), "--terminate-after", now.Add(45 * time.Minute).Format(time.RFC3339), "--qualification-evidence", evidence, "--qualification-evidence-sha256", os.Getenv("ARDENTS_FORWARDING_BUILD_EVIDENCE_SHA256"), "--output", output}
	}
	output := filepath.Join(parent, "public-plan")
	if raw, err := exec.CommandContext(t.Context(), "python3", arguments(buildRoot, descriptor, output)...).CombinedOutput(); err != nil {
		t.Fatal("actual build-bound public plan", err, string(raw))
	}
	planPath := filepath.Join(output, "forwarding-release-plan.json")
	planBytes := read(planPath)
	plan, err := decodeForwardingReleasePurpose(planBytes, time.Now().UTC(), forwardingReleaseInitial)
	if err != nil || plan.Program != filepath.Join(buildRoot, "build-1", "ardents-node") || plan.Descriptor != descriptor || plan.Template != template {
		t.Fatal("public plan changed exact canonical resources", err)
	}
	keyRoot := filepath.Join(parent, "release-keys")
	call("prepare-release-keys", keyRoot)
	keyFacts := map[string]os.FileInfo{}
	for i := 1; i <= 5; i++ {
		path := filepath.Join(keyRoot, fmt.Sprintf("release-key-%d.pem", i))
		keyFacts[path], err = os.Lstat(path)
		if err != nil || keyFacts[path].Mode() != 0600 {
			t.Fatal("original private key custody", err)
		}
	}
	metadataRoot := filepath.Join(parent, "metadata")
	call("sign-initial-release", planPath, keyRoot, metadataRoot, filepath.Join(parent, "verification"))
	root, err := metadata.Root().FromBytes(read(filepath.Join(metadataRoot, "root.json")))
	if err != nil {
		t.Fatal(err)
	}
	targets, err := metadata.Targets().FromBytes(read(filepath.Join(metadataRoot, "targets.json")))
	if err != nil || root.VerifyDelegate("targets", targets) != nil || len(targets.Signed.Targets) != 2 {
		t.Fatal("actual build-bound target signatures", err)
	}
	for name, raw := range map[string][]byte{infrastructure.ProgramTarget: nodeBytes, infrastructure.ForwardingGenerationTarget: read(descriptor)} {
		target := targets.Signed.Targets[name]
		sum := sha256.Sum256(raw)
		if target == nil || !bytes.Equal(target.Hashes["sha256"], sum[:]) {
			t.Fatal("signed target differs from actual resource", name)
		}
	}
	for path, before := range keyFacts {
		after, err := os.Lstat(path)
		if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
			t.Fatal("private key replaced or changed", err)
		}
	}
	if raw, err := exec.CommandContext(t.Context(), "python3", arguments(buildRoot, descriptor, output)...).CombinedOutput(); err == nil || !bytes.Equal(read(planPath), planBytes) {
		t.Fatal("existing public output adopted or changed", err, string(raw))
	}
	for _, fault := range []string{"source-record", "changed-command", "independent-claim", "changed-sbom", "writable-root"} {
		t.Run(fault, func(t *testing.T) {
			copyRoot := filepath.Join(parent, fault+"-build")
			if err := os.CopyFS(copyRoot, os.DirFS(buildRoot)); err != nil {
				t.Fatal(err)
			}
			name := "build-2.json"
			switch fault {
			case "changed-command":
				name = "build-2/ardents-node"
			case "independent-claim":
				name = "READY"
			case "changed-sbom":
				name = "sbom.json"
			}
			path := filepath.Join(copyRoot, name)
			raw := read(path)
			switch fault {
			case "source-record":
				raw = bytes.ReplaceAll(raw, []byte("ardents-forwarding-build-execution-v1"), []byte("ardents-source-build-execution-v1"))
			case "independent-claim":
				raw = bytes.ReplaceAll(raw, []byte(`"independent_builders":false`), []byte(`"independent_builders":true`))
			case "writable-root":
				if err := os.Chmod(copyRoot, 0777); err != nil {
					t.Fatal(err)
				}
			default:
				raw = append(raw, '\n')
			}
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			refused := filepath.Join(parent, fault+"-output")
			if result, err := exec.CommandContext(t.Context(), "python3", arguments(copyRoot, descriptor, refused)...).CombinedOutput(); err == nil {
				t.Fatal("damaged actual build record admitted", string(result))
			}
			if _, err := os.Lstat(refused); !os.IsNotExist(err) {
				t.Fatal("refused public input created output", err)
			}
		})
	}
	for _, version := range []int64{1, 2} {
		raw, err := infrastructure.PrepareDescriptor("controlled-forwarding-build", version, nodeBytes, read(template))
		if version == 2 {
			raw, err = infrastructure.PrepareForwardingDescriptor("controlled-forwarding-build", version, nodeBytes, read(template))
		}
		if err != nil {
			t.Fatal(err)
		}
		foreign := filepath.Join(parent, fmt.Sprintf("foreign-descriptor-%d.json", version))
		if err := os.WriteFile(foreign, raw, 0600); err != nil {
			t.Fatal(err)
		}
		refused := foreign + "-output"
		if result, err := exec.CommandContext(t.Context(), "python3", arguments(buildRoot, foreign, refused)...).CombinedOutput(); err == nil {
			t.Fatal("foreign invocation or noninitial version admitted", string(result))
		}
		if _, err := os.Lstat(refused); !os.IsNotExist(err) {
			t.Fatal("foreign descriptor created output", err)
		}
	}
}
