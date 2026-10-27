//go:build linux && network_signing_native

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
)

func TestForwardingEpochProvisionNativeCanonicalRecordsSignatureAndSelectedProof(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	command, _ := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_CONTROL_PROGRAM")
	call := func(args ...string) []byte {
		t.Helper()
		output, err := exec.CommandContext(t.Context(), command, append([]string{"forwarding"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal("canonical Epoch preparation refused", args[0], err, string(output))
		}
		return output
	}
	digest := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	now := time.Now().UTC().Truncate(time.Second)
	stateRoot := filepath.Join(parent, "state-key")
	var stateKey struct {
		Operation string `json:"operation"`
		Public    string `json:"state_public_key"`
		State     bool   `json:"state_accepted"`
		Runtime   bool   `json:"runtime_authority"`
	}
	if json.Unmarshal(call("prepare-state-key", stateRoot), &stateKey) != nil || stateKey.Operation != "forwarding.prepare-state-key" || len(stateKey.Public) != 64 || stateKey.State || stateKey.Runtime {
		t.Fatal("State key receipt supplies unexpected authority")
	}
	args := []string{"--network-id", strings.Repeat("01", 32), "--authority-key", stateKey.Public, "--assignment-seed", strings.Repeat("04", 32), "--node-id", strings.Repeat("03", 32), "--valid-from", now.Add(-time.Minute).Format(time.RFC3339), "--valid-until", now.Add(time.Hour).Format(time.RFC3339), "--domain", "a", "--domain", "b"}
	var nodeKey string
	for _, id := range []string{strings.Repeat("03", 32), strings.Repeat("02", 32)} {
		keyRoot := filepath.Join(parent, "node-key-"+id[:2])
		var key struct {
			Public string `json:"node_public_key"`
		}
		if json.Unmarshal(call("prepare-node-key", keyRoot), &key) != nil || len(key.Public) != 64 || key.Public == stateKey.Public {
			t.Fatal("Node/State keys collapsed")
		}
		nodeKey = filepath.Join(keyRoot, "node-key.pem")
		port := "9402"
		if id == strings.Repeat("03", 32) {
			port = "9403"
		}
		planRoot := filepath.Join(parent, "record-plan-"+id[:2])
		call("prepare-node-record", "--network-id", strings.Repeat("01", 32), "--node-id", id, "--node-key", key.Public, "--family", "one-controlled-family", "--endpoint", "127.0.0.1:"+port, "--valid-from", now.Add(-2*time.Minute).Format(time.RFC3339), "--valid-until", now.Add(2*time.Hour).Format(time.RFC3339), "--capacity", "1", "--output", planRoot)
		message, err := os.ReadFile(filepath.Join(planRoot, "node-record.signing-input"))
		if err != nil {
			t.Fatal(err)
		}
		recordRoot := filepath.Join(parent, "record-"+id[:2])
		call("sign-node-record", filepath.Join(planRoot, "node-record-plan.json"), filepath.Join(planRoot, "node-record.signing-input"), digest(message), nodeKey, recordRoot)
		args = append(args, "--record-plan", filepath.Join(planRoot, "node-record-plan.json"), "--record", filepath.Join(recordRoot, "node-record.bin"))
	}
	planRoot := filepath.Join(parent, "epoch-plan")
	call(append([]string{"prepare-epoch"}, append(append([]string(nil), args...), "--output", planRoot)...)...)
	planPath, preparedPath := filepath.Join(planRoot, "epoch-plan.json"), filepath.Join(planRoot, "epoch.unsigned")
	prepared, err := os.ReadFile(preparedPath)
	if err != nil {
		t.Fatal(err)
	}
	signedRoot := filepath.Join(parent, "epoch-signed")
	statePrivate := filepath.Join(stateRoot, "state-key.pem")
	before, err := os.Lstat(statePrivate)
	if err != nil || before.Mode() != 0600 {
		t.Fatal("original State key custody unavailable", err)
	}
	call("sign-epoch", planPath, preparedPath, digest(prepared), statePrivate, signedRoot)
	bundleRaw, err := os.ReadFile(filepath.Join(signedRoot, "state-bundle.bin"))
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := source.DecodeBundle(bundleRaw)
	if err != nil || len(bundle.Materials) != 1 || len(bundle.Inputs) != 2 {
		t.Fatal("Epoch bundle inventory differs", err)
	}
	public, err := hex.DecodeString(stateKey.Public)
	if err != nil {
		t.Fatal(err)
	}
	keyID := sha256.Sum256(public)
	policy := epoch.Policy{NetworkID: [32]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, Authorities: map[[32]byte]ed25519.PublicKey{keyID: public}, Threshold: 1, Profile: epoch.ProfileClosedRoute, Now: time.Now().UTC(), MaterializationIndex: 1}
	decision, err := epoch.Verify(policy, bundle.Epoch, bundle.Inputs, bundle.Materials, true)
	if err != nil || !decision.Snapshot.RecordPresent || hex.EncodeToString(decision.Snapshot.NodeID[:]) != strings.Repeat("03", 32) {
		t.Fatal("exact nonzero selected materialization failed", err)
	}
	index, err := epoch.InspectMaterializationIndex(bundle.Materials[0])
	if err != nil || index != 1 {
		t.Fatal("selected proof silently defaulted to first candidate", index, err)
	}
	if material, err := os.ReadFile(filepath.Join(signedRoot, "material.bin")); err != nil || !bytes.Equal(material, bundle.Materials[0]) {
		t.Fatal("public proof output differs", err)
	}
	if output, err := exec.CommandContext(t.Context(), command, "forwarding", "sign-epoch", planPath, preparedPath, digest(prepared), nodeKey, filepath.Join(parent, "wrong-key")).CombinedOutput(); err == nil || bytes.Contains(output, []byte(`"runtime_authority"`)) {
		t.Fatal("Node key admitted as State signer", err)
	}
	if _, err := os.Lstat(filepath.Join(parent, "wrong-key")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("wrong State key created output", err)
	}
	for _, mode := range []string{"pin", "message"} {
		input, pin := preparedPath, strings.Repeat("00", 32)
		if mode == "message" {
			bad := bytes.Clone(prepared)
			bad[len(bad)-1] ^= 1
			input = filepath.Join(parent, "changed-epoch")
			if err := os.WriteFile(input, bad, 0600); err != nil {
				t.Fatal(err)
			}
			pin = digest(bad)
		}
		output := filepath.Join(parent, "refused-"+mode)
		if _, err := signForwardingEpoch(t.Context(), []string{planPath, input, pin, filepath.Join(parent, "unopened-private-key"), output}, nil); err == nil || errors.Is(err, os.ErrNotExist) {
			t.Fatal("changed public Epoch reached private key", mode, err)
		}
		if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("refused Epoch created output", mode, err)
		}
	}
	if after, err := os.Lstat(statePrivate); err != nil || !os.SameFile(before, after) || after.Mode() != before.Mode() || after.ModTime() != before.ModTime() {
		t.Fatal("State key replaced", err)
	}
	if _, err := exec.CommandContext(t.Context(), command, "forwarding", "prepare-state-key", stateRoot).CombinedOutput(); err == nil {
		t.Fatal("existing State key output adopted")
	}
}
