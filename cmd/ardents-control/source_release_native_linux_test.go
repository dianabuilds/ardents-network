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

	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
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

// Fresh real signatures and actual canonical binaries exercise this closed
// offline purpose. Fixture build/qualification declarations are controlled
// inputs, not operational rebuild evidence or installed qualification.
func TestSourcePreparationNativeInitialReleasePurposeAndKeyCustody(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	command, controlBytes := sourceNativePinnedProgram(t, "ARDENTS_SOURCE_CONTROL_PROGRAM")
	_, nodeBytes := sourceNativePinnedProgram(t, "ARDENTS_SOURCE_NODE_PROGRAM")
	templateOutput := filepath.Join(parent, "template-output")
	if out, err := exec.CommandContext(t.Context(), command, "source", "prepare-template", templateOutput).CombinedOutput(); err != nil {
		t.Fatal("actual fixed template producer", string(out), err)
	}
	preparedTemplate, err := os.ReadFile(filepath.Join(templateOutput, "ardents-source.service"))
	if err != nil || !bytes.Equal(preparedTemplate, []byte(infrastructure.SourceTemplate)) {
		t.Fatal("closed template differs", err)
	}
	program, template, descriptor := filepath.Join(parent, "node"), filepath.Join(parent, "template"), filepath.Join(parent, "descriptor")
	for path, raw := range map[string][]byte{program: nodeBytes, template: preparedTemplate} {
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	descriptorBytes, err := infrastructure.PrepareDescriptor("one", 1, nodeBytes, []byte(infrastructure.SourceTemplate))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(descriptor, descriptorBytes, 0600); err != nil {
		t.Fatal(err)
	}
	keyRoot := filepath.Join(parent, "keys")
	keyOutput, err := exec.CommandContext(t.Context(), command, "source", "prepare-release-keys", keyRoot).CombinedOutput()
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
	if _, err := prepareSourceReleaseKeys(t.Context(), keyRoot, nil); err == nil {
		t.Fatal("Release key prefix adopted")
	}
	keyCtx, keyCancel := context.WithCancel(t.Context())
	defer keyCancel()
	failedPublic, keyErr := prepareSourceReleaseKeys(keyCtx, filepath.Join(parent, "failed-keys"), func(stage string, file *os.File) {
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
	plan := sourceReleasePlan{Schema: "ardents-source-initial-release-plan-v1", Network: "one-network", Reference: now, Expires: now.Add(time.Hour), Program: program, Descriptor: descriptor, Template: template, ProgramCustom: custom(nodeBytes), GenerationCustom: custom(descriptorBytes)}
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
	child := exec.CommandContext(t.Context(), command, "source", "sign-initial-release", planPath, keyRoot, output, verification)
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
	// A real Control ELF cannot be substituted for the Node target, even with
	// matching descriptor hashes and otherwise accepted fixture declarations.
	if err := os.WriteFile(program, controlBytes, 0600); err != nil {
		t.Fatal(err)
	}
	badDescriptor, err := infrastructure.PrepareDescriptor("one", 1, controlBytes, []byte(infrastructure.SourceTemplate))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(descriptor, badDescriptor, 0600); err != nil {
		t.Fatal(err)
	}
	plan.ProgramCustom, plan.GenerationCustom = custom(controlBytes), custom(badDescriptor)
	writePlan()
	signed := false
	if raw, err := signInitialSourceRelease(t.Context(), []string{planPath, keyRoot, filepath.Join(parent, "wrong-program"), filepath.Join(parent, "wrong-verification")}, func(string, *os.File) { signed = true }); err == nil || raw != nil || signed {
		t.Fatal("wrong executable reached signing", err, signed)
	}
	if err := os.WriteFile(program, nodeBytes, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(descriptor, descriptorBytes, 0600); err != nil {
		t.Fatal(err)
	}
	plan.ProgramCustom, plan.GenerationCustom = custom(nodeBytes), custom(descriptorBytes)
	writePlan()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	seen := false
	raw, err := signInitialSourceRelease(ctx, []string{planPath, keyRoot, filepath.Join(parent, "failed-metadata"), filepath.Join(parent, "failed-verification")}, func(stage string, file *os.File) {
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
