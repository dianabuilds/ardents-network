//go:build linux && network_signing_native

package main

import (
	"crypto/ed25519"
	"crypto/rand"
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

// This private component input has fixed test-family declarations and one fixed
// seed, not operational independence or a family/seed search. Every successful
// Record/Epoch/key/profile producer below is the actual canonical command; this
// harness never reads a private key or builds a signed-byte fixture itself.
func prepareForwardingStateNativeInputs(t *testing.T, command, parent string) (bundle, profile []byte, authority [32]byte) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clear(private)
	var permission [32]byte
	copy(permission[:], public)
	return prepareForwardingStateNativeInputsWithPermission(t, command, parent, permission)
}

func prepareForwardingStateNativeInputsWithPermission(t *testing.T, command, parent string, permission [32]byte) (bundle, profile []byte, authority [32]byte) {
	t.Helper()
	bundle, profile, authority, _ = prepareForwardingStateNativeCohort(t, command, parent, permission, false)
	return bundle, profile, authority
}

func prepareForwardingStateNativeCohort(t *testing.T, command, parent string, permission [32]byte, interior bool) (bundle, profile []byte, authority [32]byte, peer string) {
	t.Helper()
	if permission == [32]byte{} {
		t.Fatal("explicit public Permission authority absent")
	}
	preparation := filepath.Join(parent, "canonical-preparation")
	if err := os.Mkdir(preparation, 0700); err != nil {
		t.Fatal(err)
	}
	call := func(args ...string) []byte {
		t.Helper()
		raw, err := exec.CommandContext(t.Context(), command, append([]string{"forwarding"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal("actual canonical State preparation input", args[0], err, string(raw))
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
	digest := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	network, seed := [32]byte{1}, [32]byte{13}
	issuerID, forwardingID := [32]byte{2}, [32]byte{3}
	issuerText, forwardingText := hex.EncodeToString(issuerID[:]), hex.EncodeToString(forwardingID[:])
	from := time.Now().UTC().Truncate(time.Hour)
	until := from.Add(2 * time.Hour)
	stateRoot := filepath.Join(preparation, "state-key")
	var state struct {
		Public string `json:"state_public_key"`
	}
	if json.Unmarshal(call("prepare-state-key", stateRoot), &state) != nil {
		t.Fatal("canonical State-key receipt")
	}
	pub, err := hex.DecodeString(state.Public)
	if err != nil || len(pub) != 32 {
		t.Fatal("canonical State public key unavailable", err)
	}
	copy(authority[:], pub)
	epochArgs := []string{"prepare-epoch", "--network-id", hex.EncodeToString(network[:]), "--authority-key", state.Public, "--assignment-seed", hex.EncodeToString(seed[:]), "--node-id", issuerText, "--valid-from", from.Format(time.RFC3339), "--valid-until", until.Format(time.RFC3339), "--domain", "initiator", "--domain", "introduction", "--domain", "rendezvous", "--domain", "responder"}
	var issuerPublic string
	nodes := []struct{ id, family, endpoint string }{{issuerText, "closed-0-0", "127.0.0.1:4401"}, {forwardingText, "closed-1-0", "127.0.0.1:4402"}}
	if interior {
		// One fixed declared cohort and seed, all records retained. Selection
		// follows the authenticated assignment; no family/seed redraw or claim
		// of independently operated families is made by this test preparation.
		for index := range 16 {
			id := [32]byte{byte(16 + index)}
			nodes = append(nodes, struct{ id, family, endpoint string }{hex.EncodeToString(id[:]), fmt.Sprintf("closed-forwarding-peer-%02d", index), fmt.Sprintf("127.0.0.1:%d", 4500+index)})
		}
	}
	for index, node := range nodes {
		keyRoot := filepath.Join(preparation, "node-key-"+node.id[:2])
		var key struct {
			Public string `json:"node_public_key"`
		}
		if json.Unmarshal(call("prepare-node-key", keyRoot), &key) != nil || len(key.Public) != 64 {
			t.Fatal("canonical Node-key receipt")
		}
		if index == 0 {
			issuerPublic = key.Public
		}
		planRoot := filepath.Join(preparation, "record-plan-"+node.id[:2])
		signedRoot := filepath.Join(preparation, "record-"+node.id[:2])
		call("prepare-node-record", "--network-id", hex.EncodeToString(network[:]), "--node-id", node.id, "--node-key", key.Public, "--family", node.family, "--endpoint", node.endpoint, "--valid-from", from.Format(time.RFC3339), "--valid-until", until.Format(time.RFC3339), "--capacity", "3", "--output", planRoot)
		message := filepath.Join(planRoot, "node-record.signing-input")
		call("sign-node-record", filepath.Join(planRoot, "node-record-plan.json"), message, digest(read(message)), filepath.Join(keyRoot, "node-key.pem"), signedRoot)
		epochArgs = append(epochArgs, "--record-plan", filepath.Join(planRoot, "node-record-plan.json"), "--record", filepath.Join(signedRoot, "node-record.bin"))
	}
	epochPlan := filepath.Join(preparation, "epoch-plan")
	call(append(epochArgs, "--output", epochPlan)...)
	message := filepath.Join(epochPlan, "epoch.unsigned")
	epochRoot := filepath.Join(preparation, "epoch")
	call("sign-epoch", filepath.Join(epochPlan, "epoch-plan.json"), message, digest(read(message)), filepath.Join(stateRoot, "state-key.pem"), epochRoot)
	tokenRoot := filepath.Join(preparation, "token-material")
	call("prepare-token-keys", "--network-id", hex.EncodeToString(network[:]), "--issuer-node-id", issuerText, "--issuer-node-key", issuerPublic, "--start", from.Format(time.RFC3339), "--end", until.Format(time.RFC3339), "--output", tokenRoot)
	profilePlan := filepath.Join(preparation, "profile-plan")
	bundlePath := filepath.Join(epochRoot, "state-bundle.bin")
	profileArgs := []string{"prepare-profile", "--network-id", hex.EncodeToString(network[:]), "--authority-key", state.Public, "--issuance-authority-key", hex.EncodeToString(permission[:]), "--bundle", bundlePath, "--token-inventory", filepath.Join(tokenRoot, "public", "token-inventory.json"), "--duty", issuerText + ":2:6", "--duty", forwardingText + ":4:1"}
	if interior {
		peer = forwardingCanonicalInteriorCandidate(t, read(bundlePath), authority)
		profileArgs = append(profileArgs, "--duty", peer+":4:2")
	}
	call(append(profileArgs, "--output", profilePlan)...)
	profileMessage := filepath.Join(profilePlan, "profile.unsigned")
	profileRoot := filepath.Join(preparation, "profile")
	call("sign-profile", filepath.Join(profilePlan, "profile-plan.json"), profileMessage, digest(read(profileMessage)), filepath.Join(stateRoot, "state-key.pem"), profileRoot)
	return read(bundlePath), read(filepath.Join(profileRoot, "profile.bin")), authority, peer
}
