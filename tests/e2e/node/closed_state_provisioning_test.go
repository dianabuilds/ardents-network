package state_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// Canonical test inputs still pass the production Epoch, Node Record and
// durable State validators. No accepted projection is fabricated here.
func closedProvisioningState(t *testing.T, network, issuer [32]byte, authority, issuerKey ed25519.PrivateKey, now time.Time, carrier string) (state.Config, state.Snapshot, []Record, string, []string) {
	t.Helper()
	records := make([]Record, 3)
	for index, key := range []ed25519.PrivateKey{ed25519.NewKeyFromSeed(bytes.Repeat([]byte{6}, ed25519.SeedSize)), issuerKey, ed25519.NewKeyFromSeed(bytes.Repeat([]byte{8}, ed25519.SeedSize))} {
		node := network
		if index == 1 {
			node = issuer
		}
		if index == 2 {
			node = [32]byte{3}
		}
		record, err := BuildRecord(RecordSpec{NetworkID: network, NodeID: node, Generation: uint64(index + 1),
			ValidFrom: now, ValidUntil: now.Add(2 * time.Hour), Family: []string{"first", "issuer", "interior"}[index],
			Endpoint: freeAddress(t), Carrier: carrier,
			Capability: 2, Capacity: 4, PrivateKey: key})
		if err != nil {
			t.Fatal(err)
		}
		records[index] = record
	}
	epoch, err := BuildEpoch(EpochSpec{NetworkID: network, Number: 1, ValidFrom: now, ValidUntil: now.Add(2 * time.Hour),
		Inputs: [][]byte{records[0].Raw, records[1].Raw, records[2].Raw}, Accepted: records, AssignmentSeed: sha256.Sum256([]byte("closed command provisioning")),
		Profile: "ardents-route-v3", Version: 3, Domains: []string{"alpha", "beta"}, Authorities: []ed25519.PrivateKey{authority}})
	if err != nil {
		t.Fatal(err)
	}
	public := authority.Public().(ed25519.PublicKey)
	config := state.Config{Root: t.TempDir(), NetworkID: network, Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public},
		Threshold: 1, ClosedProfileAuthority: public, AcceptedProfile: "ardents-route-v3", Now: time.Now().UTC(), ClockObservation: time.Now().UTC()}
	binary := buildCommand(t, "ardents")
	directory := t.TempDir()
	inputs := filepath.Join(directory, "inputs")
	if err := os.Mkdir(inputs, 0o700); err != nil {
		t.Fatal(err)
	}
	epochPath, materialPath := filepath.Join(directory, "epoch.bin"), filepath.Join(directory, "material.bin")
	files := map[string][]byte{epochPath: epoch.Raw, materialPath: epoch.Materials[0], materialPath + ".1": epoch.Materials[1], materialPath + ".2": epoch.Materials[2]}
	for index, raw := range epoch.Inputs {
		files[filepath.Join(inputs, fmt.Sprintf("%04d.bin", index))] = raw
	}
	for path, raw := range files {
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	arguments := []string{"accept-offline", "--state-root", config.Root, "--network-id", hex.EncodeToString(network[:]),
		"--authorities", hex.EncodeToString(public), "--threshold", "1", "--at", config.Now.Format(time.RFC3339),
		"--epoch", epochPath, "--inputs", inputs, "--materialization", materialPath, "--profile", config.AcceptedProfile,
		"--closed-profile-authority", hex.EncodeToString(public)}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, binary, arguments...).CombinedOutput(); err != nil {
		t.Fatalf("command acceptance of canonical closed State: %v / %s", err, output)
	}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, acceptErr := owner.Current()
	closeErr := owner.Close()
	if acceptErr != nil || closeErr != nil {
		t.Fatalf("accept canonical closed State: %v / %v", acceptErr, closeErr)
	}
	return config, snapshot, records, binary, arguments
}

func runProvisioningCommand(t *testing.T, binary string, arguments ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, binary, arguments...).CombinedOutput(); err != nil {
		t.Fatalf("provisioning command %s: %v / %s", arguments[0], err, output)
	}
}
