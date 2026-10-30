//go:build linux && network_signing_native

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/forwarding"
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

// This produces public successor deliveries only. Genuine predecessor retirement,
// selection, semantic-history inspections and each installed start stay separate.
func prepareForwardingSuccessorDeliveries(t *testing.T, delivery, previous, keys, requestPath, planPath string, staleSuccessor bool) {
	t.Helper()
	command, controlBytes := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_SUCCESSOR_CONTROL_PROGRAM")
	program, nodeBytes := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_SUCCESSOR_NODE_PROGRAM")
	build := os.Getenv("ARDENTS_FORWARDING_SUCCESSOR_BUILD_ROOT")
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
	if !filepath.IsAbs(build) || digest(read(filepath.Join(build, "READY"))) != os.Getenv("ARDENTS_FORWARDING_SUCCESSOR_BUILD_READY_SHA256") {
		t.Fatal("invalid environment: genuine successor build commitment required")
	}
	for _, number := range []string{"build-1", "build-2"} {
		if !bytes.Equal(nodeBytes, read(filepath.Join(build, number, "ardents-node"))) || !bytes.Equal(controlBytes, read(filepath.Join(build, number, "ardents-control"))) {
			t.Fatal("successor commands differ from their actual build records")
		}
	}
	if bytes.Equal(nodeBytes, read(filepath.Join(previous, "ardents-node-linux-amd64"))) {
		t.Fatal("program replacement requires genuinely different canonical program bytes")
	}
	originalRoot := read(filepath.Join(previous, "root.json"))
	originalTemplate := read(filepath.Join(previous, "ardents-forwarding.service"))
	request, err := forwarding.DecodeRequest(read(requestPath))
	if err != nil {
		t.Fatal(err)
	}
	initialPlanRaw := read(planPath)
	initialPlan, err := forwarding.DecodePlan(initialPlanRaw)
	if err != nil || request.MatchPlan(initialPlan) != nil {
		t.Fatal("original successor input binding", err)
	}
	call := func(args ...string) {
		t.Helper()
		if raw, err := exec.CommandContext(t.Context(), command, append([]string{"forwarding"}, args...)...).CombinedOutput(); err != nil {
			t.Fatal("actual forward delivery preparation", args[0], err, string(raw))
		}
	}
	// The signer independently retains its complete predecessor distribution.
	// Candidate resources must be outside that original input namespace.
	templateRoot := filepath.Join(delivery, "successor-template")
	call("prepare-template", templateRoot)
	template := filepath.Join(templateRoot, "ardents-forwarding.service")
	if !bytes.Equal(originalTemplate, read(template)) {
		t.Fatal("successor changed the original fixed protection template")
	}
	write := func(path string, raw []byte, mode os.FileMode) {
		t.Helper()
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			t.Fatal(err)
		}
		_, written := file.Write(raw)
		if err := errors.Join(written, file.Sync(), file.Close()); err != nil {
			t.Fatal(err)
		}
	}
	type successorDelivery struct {
		Version      int               `json:"version"`
		Distribution string            `json:"distribution"`
		Request      string            `json:"request"`
		Plan         string            `json:"plan"`
		PublicInputs map[string]string `json:"public_inputs"`
	}
	var deliveries []successorDelivery
	for version := 2; version <= 3; version++ {
		base := filepath.Join(delivery, fmt.Sprintf("successor-%d", version))
		if err := os.Mkdir(base, 0700); err != nil {
			t.Fatal(err)
		}
		descriptorRoot := filepath.Join(base, "descriptor")
		call("prepare-descriptor", "coherent-private-forwarding", fmt.Sprint(version), program, template, descriptorRoot)
		descriptor := filepath.Join(descriptorRoot, "infrastructure-forwarding.json")
		now := time.Now().UTC().Truncate(time.Second)
		noNewWork := now.Add(30 * time.Minute)
		if staleSuccessor && version == 2 {
			// The canonical public producer and signer authenticate this finite
			// bound while current. The separate negative installed case later
			// waits for real expiry; no signed bytes or runtime clock are edited.
			noNewWork = now.Add(90 * time.Second)
		}
		publicPlan := filepath.Join(base, "public-release-plan")
		args := []string{"-I", "-S", preparer, "--forward", "--build-root", build, "--descriptor", descriptor, "--template", template, "--network", "coherent-private-forwarding", "--expires", now.Add(time.Hour).Format(time.RFC3339), "--no-new-work-after", noNewWork.Format(time.RFC3339), "--terminate-after", now.Add(45 * time.Minute).Format(time.RFC3339), "--qualification-evidence", evidence, "--qualification-evidence-sha256", os.Getenv("ARDENTS_FORWARDING_BUILD_EVIDENCE_SHA256"), "--output", publicPlan}
		if raw, err := exec.CommandContext(t.Context(), "python3", args...).CombinedOutput(); err != nil {
			t.Fatal("actual successor build-bound plan", err, string(raw))
		}
		metadata := filepath.Join(base, "metadata")
		call("sign-forward-release", filepath.Join(publicPlan, "forwarding-release-plan.json"), keys, metadata, filepath.Join(base, "verification"), previous)
		if !time.Now().Before(noNewWork) {
			t.Fatal("invalid environment: actual canonical successor signing outlived its prepared lease")
		}
		if !bytes.Equal(originalRoot, read(filepath.Join(metadata, "root.json"))) {
			t.Fatal("successor changed the independently pinned initial root")
		}
		distribution := filepath.Join(base, "distribution")
		if err := os.Mkdir(distribution, 0700); err != nil {
			t.Fatal(err)
		}
		write(filepath.Join(distribution, "ardents-node-linux-amd64"), nodeBytes, 0700)
		write(filepath.Join(distribution, "ardents-forwarding.service"), read(template), 0600)
		write(filepath.Join(distribution, "infrastructure-forwarding.json"), read(descriptor), 0600)
		for _, name := range []string{"root.json", "targets.json", "snapshot.json", "timestamp.json"} {
			write(filepath.Join(distribution, name), read(filepath.Join(metadata, name)), 0600)
		}
		inventory := filepath.Join(base, "inventory")
		call("prepare-inventory", distribution, inventory)
		write(filepath.Join(distribution, "infrastructure-enrollment.json"), read(filepath.Join(inventory, "infrastructure-enrollment.json")), 0600)
		requestRoot, planRoot := filepath.Join(base, "request"), filepath.Join(base, "plan")
		call("prepare-request", "--installation-root", request.InstallationRoot, "--state-root", request.StateRoot, "--participation-root", request.ParticipationRoot, "--spend-root", request.SpendRoot, "--hosting-root", request.HostingRoot, "--credential-directory", request.CredentialDirectory, "--plan", filepath.Join(planRoot, "forwarding-plan.json"), "--certificate", request.Certificate, "--private-key", request.PrivateKey, "--clock", request.Clock, "--output", requestRoot)
		work, termination := initialPlan.Work, initialPlan.Termination
		if version == 3 {
			work.Tx += 1 << 20
			termination.Tx += 1 << 16
		}
		network := initialPlan.Network
		planArgs := []string{"prepare-plan", "--request", filepath.Join(requestRoot, "forwarding-request.json"), "--output", planRoot, "--network-id", hex.EncodeToString(network.NetworkID[:]), "--node-id", hex.EncodeToString(initialPlan.NodeID[:]), "--profile-authority", hex.EncodeToString(network.ProfileAuthority[:]), "--threshold", fmt.Sprint(network.Threshold), "--work-tx", fmt.Sprint(work.Tx), "--work-rx", fmt.Sprint(work.Rx), "--termination-tx", fmt.Sprint(termination.Tx), "--termination-rx", fmt.Sprint(termination.Rx), "--retirement-wait-ms", fmt.Sprint(initialPlan.RetirementWaitMS)}
		for _, authority := range network.Authorities {
			planArgs = append(planArgs, "--authority", hex.EncodeToString(authority[:]))
		}
		call(planArgs...)
		newPlanPath := filepath.Join(planRoot, "forwarding-plan.json")
		newRaw := read(newPlanPath)
		newPlan, err := forwarding.DecodePlan(newRaw)
		if err != nil {
			t.Fatal(err)
		}
		if version == 2 && !bytes.Equal(newRaw, initialPlanRaw) {
			t.Fatal("program-only replacement changed actual resource or identity inputs")
		}
		if version == 3 && (newPlan.Work == initialPlan.Work || newPlan.Termination == initialPlan.Termination || !bytes.Equal(nodeBytes, read(filepath.Join(previous, "ardents-node-linux-amd64")))) {
			t.Fatal("resource-only replacement lacks actual work/termination change or changed its program")
		}
		newPlan.Work, newPlan.Termination = initialPlan.Work, initialPlan.Termination
		restored, err := json.Marshal(newPlan)
		if err != nil || !bytes.Equal(append(restored, '\n'), initialPlanRaw) {
			t.Fatal("successor changed non-resource runtime declarations", err)
		}
		entry := successorDelivery{Version: version, Distribution: distribution, Request: filepath.Join(requestRoot, "forwarding-request.json"), Plan: newPlanPath, PublicInputs: map[string]string{}}
		for _, name := range []string{"ardents-node-linux-amd64", "ardents-forwarding.service", "infrastructure-forwarding.json", "infrastructure-enrollment.json", "root.json", "targets.json", "snapshot.json", "timestamp.json"} {
			path := filepath.Join(distribution, name)
			entry.PublicInputs[path] = digest(read(path))
		}
		entry.PublicInputs[entry.Request], entry.PublicInputs[entry.Plan] = digest(read(entry.Request)), digest(newRaw)
		deliveries = append(deliveries, entry)
		previous = distribution
	}
	raw, err := json.Marshal(struct {
		Schema     string              `json:"schema"`
		Deliveries []successorDelivery `json:"deliveries"`
		Installed  bool                `json:"installed"`
	}{Schema: "ardents-forwarding-successor-deliveries-v1", Deliveries: deliveries})
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(delivery, "successor-deliveries.json"), append(raw, '\n'), 0600)
	directory, err := os.Open(delivery)
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(directory.Sync(), directory.Close()); err != nil {
		t.Fatal(err)
	}
}
