//go:build linux && network_signing_native

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
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

	"github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
)

func TestForwardingProfileNativeCanonicalEpochInventoryAndSignature(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	command, _ := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_CONTROL_PROGRAM")
	call := func(args ...string) []byte {
		t.Helper()
		raw, err := exec.CommandContext(t.Context(), command, append([]string{"forwarding"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal("canonical public profile flow refused", args[0], err, string(raw))
		}
		return raw
	}
	digest := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	stateRoot := filepath.Join(parent, "state-key")
	var state struct {
		Public string `json:"state_public_key"`
	}
	if json.Unmarshal(call("prepare-state-key", stateRoot), &state) != nil || len(state.Public) != 64 {
		t.Fatal("State public key unavailable")
	}
	start := time.Now().UTC().Truncate(time.Hour)
	end := start.Add(time.Hour)
	network := strings.Repeat("01", 32)
	issuerID, otherID := strings.Repeat("02", 32), strings.Repeat("03", 32)
	epochArgs := []string{"prepare-epoch", "--network-id", network, "--authority-key", state.Public, "--assignment-seed", strings.Repeat("04", 32), "--node-id", issuerID, "--valid-from", start.Format(time.RFC3339), "--valid-until", end.Format(time.RFC3339), "--domain", "rendezvous"}
	var issuerPublic, issuerPrivate string
	for index, id := range []string{issuerID, otherID} {
		keyRoot := filepath.Join(parent, "node-key-"+id[:2])
		var key struct {
			Public string `json:"node_public_key"`
		}
		if json.Unmarshal(call("prepare-node-key", keyRoot), &key) != nil || len(key.Public) != 64 {
			t.Fatal("Node public key unavailable")
		}
		if id == issuerID {
			issuerPublic = key.Public
			issuerPrivate = filepath.Join(keyRoot, "node-key.pem")
		}
		planRoot := filepath.Join(parent, "record-plan-"+id[:2])
		signedRoot := filepath.Join(parent, "record-"+id[:2])
		endpoint := "127.0.0.1:9402"
		if index == 1 {
			endpoint = "127.0.0.1:9403"
		}
		call("prepare-node-record", "--network-id", network, "--node-id", id, "--node-key", key.Public, "--family", "one-controlled-family", "--endpoint", endpoint, "--valid-from", start.Format(time.RFC3339), "--valid-until", end.Format(time.RFC3339), "--capacity", "1", "--output", planRoot)
		message := filepath.Join(planRoot, "node-record.signing-input")
		call("sign-node-record", filepath.Join(planRoot, "node-record-plan.json"), message, digest(read(message)), filepath.Join(keyRoot, "node-key.pem"), signedRoot)
		epochArgs = append(epochArgs, "--record-plan", filepath.Join(planRoot, "node-record-plan.json"), "--record", filepath.Join(signedRoot, "node-record.bin"))
	}
	epochPlan := filepath.Join(parent, "epoch-plan")
	call(append(epochArgs, "--output", epochPlan)...)
	epochMessage := filepath.Join(epochPlan, "epoch.unsigned")
	epochRoot := filepath.Join(parent, "epoch")
	statePrivate := filepath.Join(stateRoot, "state-key.pem")
	call("sign-epoch", filepath.Join(epochPlan, "epoch-plan.json"), epochMessage, digest(read(epochMessage)), statePrivate, epochRoot)
	tokenRoot := filepath.Join(parent, "token-material")
	call("prepare-token-keys", "--network-id", network, "--issuer-node-id", issuerID, "--issuer-node-key", issuerPublic, "--start", start.Format(time.RFC3339), "--end", end.Format(time.RFC3339), "--output", tokenRoot)
	// This independently supplied test public Permission pin grants no private
	// signing, Permission or installed Issuer authority. No private fixture is
	// extracted; actual Node/State signing above uses canonical born keys only.
	permissionPublic, permissionPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clear(permissionPrivate)
	profileRoot := filepath.Join(parent, "profile-plan")
	bundlePath := filepath.Join(epochRoot, "state-bundle.bin")
	call("prepare-profile", "--network-id", network, "--authority-key", state.Public, "--issuance-authority-key", hex.EncodeToString(permissionPublic), "--bundle", bundlePath, "--token-inventory", filepath.Join(tokenRoot, "public", "token-inventory.json"), "--duty", issuerID+":2:6", "--duty", otherID+":2:4", "--output", profileRoot)
	planPath, messagePath := filepath.Join(profileRoot, "profile-plan.json"), filepath.Join(profileRoot, "profile.unsigned")
	message := read(messagePath)
	signedRoot := filepath.Join(parent, "profile-signed")
	before, err := os.Lstat(statePrivate)
	if err != nil {
		t.Fatal(err)
	}
	call("sign-profile", planPath, messagePath, digest(message), statePrivate, signedRoot)
	signed := read(filepath.Join(signedRoot, "profile.bin"))
	bundle, err := source.DecodeBundle(read(bundlePath))
	if err != nil {
		t.Fatal(err)
	}
	statePublic, err := hex.DecodeString(state.Public)
	if err != nil {
		t.Fatal(err)
	}
	var networkID [32]byte
	for i := range networkID {
		networkID[i] = 1
	}
	decision, err := epoch.Verify(epoch.Policy{NetworkID: networkID, Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(statePublic): statePublic}, Threshold: 1, Profile: epoch.ProfileClosedRoute, Now: time.Now().UTC()}, bundle.Epoch, bundle.Inputs, bundle.Materials, true)
	if err != nil {
		t.Fatal("independent original Epoch verification", err)
	}
	profile, err := closedprofile.Verify(signed, closedprofile.Context{NetworkID: networkID, StateGeneration: decision.Header.Digest, EpochDigest: decision.Header.Digest, Epoch: decision.Header.Number, Authority: statePublic, Now: time.Now().UTC()})
	if err != nil || len(profile.Nodes) != 2 || len(profile.Keys) != 3 || profile.Nodes[0].RecordDigest != decision.Candidates[0].RecordDigest || profile.Nodes[1].RecordDigest != decision.Candidates[1].RecordDigest || profile.Nodes[0].DutyGeneration != 1 || profile.Nodes[1].Subrole != 4 || !bytes.Equal(profile.AuthorityKey[:], permissionPublic) {
		t.Fatal("signed profile differs from actual Epoch/inventory", err)
	}
	for index, node := range profile.Nodes {
		if node.NodeID != decision.Candidates[index].NodeID || node.RecordDigest != sha256.Sum256(bundle.Inputs[index]) {
			t.Fatal("exact signed Node reference mismatch")
		}
	}
	after, err := os.Lstat(statePrivate)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("profile signer changed original State key custody", err)
	}
	if _, _, err := signForwardingProfile(t.Context(), []string{planPath, messagePath, digest(message), issuerPrivate, filepath.Join(parent, "wrong-signer")}, nil); err == nil {
		t.Fatal("Node key used as State signer")
	}
	if _, err := os.Lstat(filepath.Join(parent, "wrong-signer")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("wrong signer created output", err)
	}
	var plan forwardingProfilePlan
	if json.Unmarshal(read(planPath), &plan) != nil {
		t.Fatal("profile plan malformed")
	}
	for _, tc := range []struct {
		name   string
		change func(*forwardingProfilePlan)
	}{
		{"schema", func(p *forwardingProfilePlan) { p.Schema = "ardents-source-plan-v1" }},
		{"stale", func(p *forwardingProfilePlan) { p.Reference = p.Reference.Add(-6 * time.Minute) }},
		{"future", func(p *forwardingProfilePlan) { p.Reference = p.Reference.Add(2 * time.Minute) }},
		{"network", func(p *forwardingProfilePlan) { p.Network = strings.Repeat("05", 32) }},
		{"state-authority-alias", func(p *forwardingProfilePlan) { p.PermissionAuthority = p.Authority }},
		{"node-authority-alias", func(p *forwardingProfilePlan) { p.PermissionAuthority = issuerPublic }},
		{"issuer-key", func(p *forwardingProfilePlan) { p.Tokens.Inventory.Binding.Signer = [32]byte{7} }},
		{"missing-class", func(p *forwardingProfilePlan) { p.Tokens.Inventory.Keys = p.Tokens.Inventory.Keys[:2] }},
		{"duplicate-duty", func(p *forwardingProfilePlan) { p.Duties[1] = p.Duties[0] }},
		{"epoch-domain-contradiction", func(p *forwardingProfilePlan) { p.Duties[1].Domain = 1; p.Duties[1].Subrole = 1 }},
		{"unknown-node", func(p *forwardingProfilePlan) { p.Duties[1].Node = strings.Repeat("06", 32) }},
		{"signed-epoch-damaged", func(p *forwardingProfilePlan) { p.Bundle[len(p.Bundle)-1] ^= 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var changed forwardingProfilePlan
			_ = json.Unmarshal(read(planPath), &changed)
			tc.change(&changed)
			raw, _ := json.Marshal(changed)
			path := filepath.Join(parent, "bad-"+tc.name)
			if err := os.WriteFile(path, append(raw, '\n'), 0600); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(parent, "refused-"+tc.name)
			if _, _, err := signForwardingProfile(t.Context(), []string{path, messagePath, digest(message), filepath.Join(parent, "unopened-private-key"), output}, nil); err == nil || errors.Is(err, os.ErrNotExist) {
				t.Fatal("invalid declaration reached private key", err)
			}
			if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("invalid plan created output", err)
			}
		})
	}
	for _, mode := range []string{"pin", "prepared"} {
		input, pin := messagePath, strings.Repeat("00", 32)
		if mode == "prepared" {
			changed := bytes.Clone(message)
			changed[len(changed)-1] ^= 1
			input = filepath.Join(parent, "changed-profile-unsigned")
			if err := os.WriteFile(input, changed, 0600); err != nil {
				t.Fatal(err)
			}
			pin = digest(changed)
		}
		if _, _, err := signForwardingProfile(t.Context(), []string{planPath, input, pin, filepath.Join(parent, "unopened-private-key"), filepath.Join(parent, "refused-"+mode)}, nil); err == nil || errors.Is(err, os.ErrNotExist) {
			t.Fatal("bad reviewed input reached private key", mode, err)
		}
	}
}
