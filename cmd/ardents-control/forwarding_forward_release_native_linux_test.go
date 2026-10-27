//go:build linux && network_signing_native

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Actual canonical programs and real fresh signatures exercise only the offline
// signing boundary. Controlled public build/qualification declarations below do
// not prove independent builds, an installed replacement or bootstrap acceptance.
func TestForwardingPreparationNativeForwardReleasePreservesRootAndRaisesFloors(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	command, _ := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_CONTROL_PROGRAM")
	program, node := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_NODE_PROGRAM")
	call := func(args ...string) []byte {
		t.Helper()
		raw, err := exec.CommandContext(t.Context(), command, append([]string{"forwarding"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal("canonical Forwarding preparation", args[0], err, string(raw))
		}
		return raw
	}
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	write := func(path string, raw []byte) {
		t.Helper()
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	keys := filepath.Join(parent, "keys")
	call("prepare-release-keys", keys)
	templateDir := filepath.Join(parent, "template")
	call("prepare-template", templateDir)
	template := filepath.Join(templateDir, "ardents-forwarding.service")
	now := time.Now().UTC().Truncate(time.Second)
	claims := func(version int, artifact []byte) []byte {
		digest := sha256.Sum256(artifact)
		attestations := []map[string]any{}
		for _, builder := range []string{"controlled-forward-one", "controlled-forward-two"} {
			attestations = append(attestations, map[string]any{"builder_identity": builder, "build_identity": "controlled-build", "source_revision": "controlled-source", "build_input_commitment": "controlled-input", "target_sha256": hex.EncodeToString(digest[:])})
		}
		raw, err := json.Marshal(map[string]any{"schema_version": 1, "profile": "ardents-h3-release-v1", "platform": "linux-amd64", "architecture": "amd64", "environment": "alpha", "network": "one-forward-network",
			"release_identity": fmt.Sprintf("forward-%d", version), "release_version": version, "source_revision": "controlled-source", "build_input_commitment": "controlled-input", "build_identity": "controlled-build", "dependency_identity": "controlled-dependencies", "sbom_identity": "controlled-sbom",
			"attestation_policy": "two-builder", "qualification": "qualified", "build_state": "current", "builder_attestations": attestations, "build_safety_no_new_work_after": now.Add(2 * time.Hour), "build_safety_terminate_after": now.Add(3 * time.Hour), "protocol_phase": "announced", "protocol_overlapped_since": time.Time{}, "capacity_ready": false, "drain_ready": false, "emergency_reason": "", "emergency_expiry": time.Time{}})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	var originalRoot []byte
	previous := ""
	for version := 1; version <= 3; version++ {
		base := filepath.Join(parent, fmt.Sprintf("version-%d", version))
		if err := os.Mkdir(base, 0700); err != nil {
			t.Fatal(err)
		}
		descriptorDir := filepath.Join(base, "descriptor")
		call("prepare-descriptor", fmt.Sprintf("forward-%d", version), fmt.Sprint(version), program, template, descriptorDir)
		descriptor := filepath.Join(descriptorDir, "infrastructure-forwarding.json")
		programClaims, generationClaims := filepath.Join(base, "program-claims"), filepath.Join(base, "generation-claims")
		write(programClaims, claims(version, node))
		write(generationClaims, claims(version, read(descriptor)))
		planDir := filepath.Join(base, "plan")
		prepare, sign := "prepare-release-plan", "sign-initial-release"
		if version > 1 {
			prepare, sign = "prepare-forward-release-plan", "sign-forward-release"
		}
		call(prepare, "--network", "one-forward-network", "--reference", now.Format(time.RFC3339), "--expires", now.Add(time.Hour).Format(time.RFC3339), "--program", program, "--descriptor", descriptor, "--template", template, "--program-custom", programClaims, "--generation-custom", generationClaims, "--output", planDir)
		plan := filepath.Join(planDir, "forwarding-release-plan.json")
		metadataDir := filepath.Join(base, "metadata")
		args := []string{sign, plan, keys, metadataDir, filepath.Join(base, "verification")}
		if version > 1 {
			args = append(args, previous)
		}
		out := call(args...)
		if !bytes.Contains(out, []byte(`"operation":"forwarding.`+sign+`"`)) {
			t.Fatal("wrong signature purpose", string(out))
		}
		root := read(filepath.Join(metadataDir, "root.json"))
		if version == 1 {
			originalRoot = bytes.Clone(root)
		} else if !bytes.Equal(root, originalRoot) {
			t.Fatal("forward signature changed the independent initial root pin")
		}
		for _, name := range []string{"targets.json", "snapshot.json", "timestamp.json"} {
			var value struct {
				Signed struct {
					Version int `json:"version"`
					Meta    map[string]struct {
						Version int `json:"version"`
					} `json:"meta"`
				} `json:"signed"`
			}
			if json.Unmarshal(read(filepath.Join(metadataDir, name)), &value) != nil || value.Signed.Version != version {
				t.Fatal("non-increasing native signed version", version, name)
			}
			if name == "snapshot.json" && value.Signed.Meta["targets.json"].Version != version || name == "timestamp.json" && value.Signed.Meta["snapshot.json"].Version != version {
				t.Fatal("metadata link lost raised version", name)
			}
		}
		distribution := filepath.Join(base, "distribution")
		if err := os.Mkdir(distribution, 0700); err != nil {
			t.Fatal(err)
		}
		for name, raw := range map[string][]byte{"ardents-node-linux-amd64": node, "ardents-forwarding.service": read(template), "infrastructure-forwarding.json": read(descriptor), "root.json": root, "targets.json": read(filepath.Join(metadataDir, "targets.json")), "snapshot.json": read(filepath.Join(metadataDir, "snapshot.json")), "timestamp.json": read(filepath.Join(metadataDir, "timestamp.json"))} {
			write(filepath.Join(distribution, name), raw)
		}
		inventory := filepath.Join(base, "inventory")
		call("prepare-inventory", distribution, inventory)
		write(filepath.Join(distribution, "infrastructure-enrollment.json"), read(filepath.Join(inventory, "infrastructure-enrollment.json")))
		if version > 1 {
			for _, fault := range []string{"same-predecessor", "changed-keys"} {
				badKeys, badPrevious := keys, distribution
				if fault == "changed-keys" {
					badKeys = filepath.Join(base, "foreign-keys")
					call("prepare-release-keys", badKeys)
					badPrevious = previous
				}
				output, verification := filepath.Join(base, "denied-"+fault), filepath.Join(base, "denied-history-"+fault)
				if raw, err := exec.CommandContext(t.Context(), command, "forwarding", "sign-forward-release", plan, badKeys, output, verification, badPrevious).CombinedOutput(); err == nil {
					t.Fatal("foreign forward signature accepted", fault, string(raw))
				}
				for _, path := range []string{output, verification} {
					if _, err := os.Lstat(path); !os.IsNotExist(err) {
						t.Fatal("denied predecessor/key admission created output", fault, path, err)
					}
				}
			}
		}
		previous = distribution
	}
}
