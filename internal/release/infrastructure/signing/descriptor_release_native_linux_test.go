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

	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
	"github.com/dianabuilds/ardents-network/internal/release/infrastructure"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestDescriptorPreparationNativeInitialReleasePurposeAndKeyCustody(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	command, controlBytes := sourceNativePinnedProgram(t, "ARDENTS_INFRASTRUCTURE_RELEASE_PROGRAM")
	nextPath, nextBytes := sourceNativePinnedProgram(t, "ARDENTS_DESCRIPTOR_NEXT_PROGRAM")
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
	descriptorBytes, err := infrastructure.PrepareRuntimeDescriptor("descriptor", "one", 1, nextBytes, []byte(systemd.PlatformUnitTemplate))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(descriptor, descriptorBytes, 0600); err != nil {
		t.Fatal(err)
	}
	keyRoot := filepath.Join(parent, "keys")
	keyOutput, err := exec.CommandContext(t.Context(), command, "descriptor", "prepare-release-keys", keyRoot).CombinedOutput()
	if err != nil {
		t.Fatal("actual Descriptor Release key birth", string(keyOutput), err)
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
	if json.Unmarshal(keyOutput, &keyReceipt) != nil || keyReceipt.Operation != "descriptor.prepare-release-keys" || keyReceipt.Threshold != 3 || keyReceipt.Independent || len(keyReceipt.Keys) != 5 {
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
	if _, err := prepareInfrastructureReleaseKeys(t.Context(), keyRoot, nil); err == nil {
		t.Fatal("Release key prefix adopted")
	}
	keyCtx, keyCancel := context.WithCancel(t.Context())
	defer keyCancel()
	failedPublic, keyErr := prepareInfrastructureReleaseKeys(keyCtx, filepath.Join(parent, "failed-keys"), func(stage string, file *os.File) {
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
	plan := fixedNodeReleasePlan{Schema: "ardents-descriptor-initial-release-plan-v1", Network: "one-network", Reference: now, Expires: now.Add(time.Hour), Program: program, Descriptor: descriptor, Template: template, ProgramCustom: custom(nextBytes), GenerationCustom: custom(descriptorBytes)}
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
	publicArgs := []string{"descriptor", "prepare-release-plan", "--network", plan.Network, "--reference", plan.Reference.Format(time.RFC3339), "--expires", plan.Expires.Format(time.RFC3339), "--program", program, "--descriptor", descriptor, "--template", template, "--program-custom", programClaims, "--generation-custom", generationClaims, "--output", planOutput}
	if out, err := exec.CommandContext(t.Context(), command, publicArgs...).CombinedOutput(); err != nil {
		t.Fatal("canonical public release plan producer", err, string(out))
	}
	planPath = filepath.Join(planOutput, "descriptor-release-plan.json")
	output, verification := filepath.Join(parent, "metadata"), filepath.Join(parent, "verification")
	child := exec.CommandContext(t.Context(), command, "descriptor", "sign-initial-release", planPath, keyRoot, output, verification)
	if out, err := child.CombinedOutput(); err != nil || !bytes.Contains(out, []byte(`"operation":"descriptor.sign-initial-release"`)) {
		t.Fatal("actual canonical Descriptor signature operation", string(out), err)
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
	if err != nil || len(targets.Signed.Targets) != 2 || targets.Signed.Targets["ardents/linux-amd64/node"] == nil || targets.Signed.Targets["ardents/linux-amd64/infrastructure/descriptor"] == nil || len(targets.Signatures) != 3 {
		t.Fatal("Descriptor target purpose changed", err)
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
			root, err := signNodeReleasePurpose(descriptorReleaseRole, ctx, []string{planPath, keyRoot, metadataPath, verificationPath}, nodeReleaseInitial, func(stage string, file *os.File) {
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
			if root, err := signNodeReleasePurpose(descriptorReleaseRole, t.Context(), []string{planPath, keyRoot, metadataPath, verificationPath}, nodeReleaseInitial, nil); err == nil || len(root) != 0 {
				t.Fatal("partial output adopted as fresh signing")
			}
			after, err := os.Stat(metadataPath)
			if err != nil || !os.SameFile(before, after) {
				t.Fatal("retained original partial output replaced", err)
			}
		})
	}

	// Neither another declared purpose nor an arbitrary executable may reach signing.
	for _, bad := range []string{"ardents-source-initial-release-plan-v1", "ardents-forwarding-initial-release-plan-v1", "ardents-forwarding-forward-release-plan-v1", "ardents-descriptor-forward-release-plan-v1"} {
		plan.Schema = bad
		writePlan()
		output := filepath.Join(parent, bad)
		if _, err := signNodeReleasePurpose(descriptorReleaseRole, t.Context(), []string{planPath, keyRoot, output, output + "-verification"}, nodeReleaseInitial, nil); err == nil {
			t.Fatal("foreign purpose reached initial signing", bad)
		}
		if _, err := os.Lstat(output); !os.IsNotExist(err) {
			t.Fatal("foreign purpose created metadata", err)
		}
	}
	plan.Schema = "ardents-descriptor-initial-release-plan-v1"
	if err := os.WriteFile(program, controlBytes, 0600); err != nil {
		t.Fatal(err)
	}
	badDescriptor, err := infrastructure.PrepareRuntimeDescriptor("descriptor", "one", 1, controlBytes, []byte(systemd.PlatformUnitTemplate))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(descriptor, badDescriptor, 0600); err != nil {
		t.Fatal(err)
	}
	plan.ProgramCustom, plan.GenerationCustom = custom(controlBytes), custom(badDescriptor)
	writePlan()
	if _, err := signNodeReleasePurpose(descriptorReleaseRole, t.Context(), []string{planPath, keyRoot, filepath.Join(parent, "bad-program"), filepath.Join(parent, "bad-program-history")}, nodeReleaseInitial, nil); err == nil {
		t.Fatal("Control executable signed as Node")
	}
}

// The former TestDescriptorPreparationNativeActualBuildRecordsAndPublicPlan retired
// with the transitional Descriptor orchestration (#604 FJD stage D, ADR-0125): its
// controlled two-execution records bound the old ardents-node program and the
// retired prepare-descriptor-release-plan.py public preparer. The platform-led
// Descriptor signing cutover keeps the canonical public prepare-release-plan
// producer and real signature coverage above; a new-path Descriptor candidate build
// and release plan builder is a separate follow-up, exactly as for Source (#604
// stage 1.6), Forwarding and JOIN (#604 FJD stage D).
