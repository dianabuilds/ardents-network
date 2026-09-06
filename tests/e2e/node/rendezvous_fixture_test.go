package state_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/tests/epochfixture/assignment"
)

func TestRendezvousStateRejectsSignedDuplicateEndpoint(t *testing.T) {
	fixture := newRendezvousStateFixtureWithPeerEndpoints(t, freeAddress(t), "127.0.0.1:17042", "127.0.0.1:17042")
	output, err := acceptRendezvousEpochCommand(t, buildCommand(t, "ardents"), t.TempDir(), fixture, 0)
	if err == nil || !strings.Contains(string(output), "candidate view or rejection commitment is inconsistent") {
		t.Fatalf("duplicate endpoint acceptance = %v\n%s", err, output)
	}
}

func newRendezvousStateFixture(t *testing.T, endpoint string) rendezvousStateFixture {
	return newRendezvousStateFixtureWithPeerEndpoints(t, endpoint, "127.0.0.1:17042", "127.0.0.1:17043")
}

func newRendezvousStateFixtureWithPeerEndpoints(t *testing.T, endpoint, initiatorEndpoint, responderEndpoint string) rendezvousStateFixture {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	network := sha256.Sum256([]byte("ardents-native-rendezvous-process-network"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x71}, ed25519.SeedSize))
	certificateAuthority := makeAuthority(t, "rendezvous-node-root")
	fixture := rendezvousStateFixture{now: now, network: network, authorityPublic: authority.Public().(ed25519.PublicKey), authorityPrivate: authority}
	fixture.rendezvous = makeRendezvousStateRecord(t, network, 0x41, "rendezvous-family", endpoint, makeLeaf(t, certificateAuthority, "rendezvous.test", true), now)
	fixture.initiator = makeRendezvousStateRecord(t, network, 0x42, "initiator-family", initiatorEndpoint, makeLeaf(t, certificateAuthority, "initiator.test", false), now)
	fixture.responder = makeRendezvousStateRecord(t, network, 0x43, "responder-family", responderEndpoint, makeLeaf(t, certificateAuthority, "responder.test", false), now)
	records := []rendezvousStateRecord{fixture.rendezvous, fixture.initiator, fixture.responder}
	sort.Slice(records, func(first, second int) bool {
		return bytes.Compare(records[first].nodeID[:], records[second].nodeID[:]) < 0
	})
	for index, record := range records {
		if record.nodeID == fixture.rendezvous.nodeID {
			fixture.rendezvousIndex = uint32(index)
			break
		}
	}
	domains := []string{"initiator", "rendezvous", "responder"}
	var seed [32]byte
	for marker := uint64(1); ; marker++ {
		seed = sha256.Sum256([]byte(fmt.Sprintf("rendezvous-process-%d", marker)))
		if rendezvousAssignments(network, 1, seed, fixture.rendezvous.family, fixture.initiator.family, fixture.responder.family) {
			break
		}
	}
	inputs, accepted := make([][]byte, len(records)), make([]Record, len(records))
	for index, record := range records {
		inputs[index] = record.raw
		accepted[index] = Record{Raw: record.raw, NodeID: record.nodeID, Family: record.family, Capacity: 4}
	}
	built, err := BuildEpoch(EpochSpec{NetworkID: network, Number: 1, ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(10 * time.Minute),
		Inputs: inputs, Accepted: accepted, AssignmentSeed: seed, Profile: route.Profile, Domains: domains, Authorities: []ed25519.PrivateKey{authority}})
	if err != nil {
		t.Fatal(err)
	}
	fixture.epoch = lifecycleEpoch{number: built.Number, seed: built.Seed, raw: built.Raw, digest: built.Digest, inputs: built.Inputs, materials: built.Materials}
	fixture.rendezvousAssignment = assignment.Digest(network, 1, seed, fixture.rendezvous.family, "rendezvous")
	return fixture
}

func acceptRendezvousEpoch(t *testing.T, binary, root string, fixture rendezvousStateFixture, materializationIndex uint32) {
	t.Helper()
	if output, err := acceptRendezvousEpochCommand(t, binary, root, fixture, materializationIndex); err != nil {
		t.Fatalf("accept native State Epoch: %v\n%s", err, output)
	}
}

func acceptRendezvousEpochCommand(t *testing.T, binary, root string, fixture rendezvousStateFixture, materializationIndex uint32) ([]byte, error) {
	t.Helper()
	directory := t.TempDir()
	inputs := filepath.Join(directory, "inputs")
	if err := os.Mkdir(inputs, 0o700); err != nil {
		t.Fatal(err)
	}
	epochPath, material := filepath.Join(directory, "epoch.bin"), filepath.Join(directory, "material.bin")
	if err := os.WriteFile(epochPath, fixture.epoch.raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(material, fixture.epoch.materials[materializationIndex], 0o600); err != nil {
		t.Fatal(err)
	}
	for index, raw := range fixture.epoch.inputs {
		if err := os.WriteFile(filepath.Join(inputs, fmt.Sprintf("%04d.bin", index)), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	arguments := []string{"accept-offline", "--state-root", root, "--network-id", hex.EncodeToString(fixture.network[:]), "--authorities", hex.EncodeToString(fixture.authorityPublic),
		"--threshold", "1", "--at", fixture.now.Format(time.RFC3339), "--epoch", epochPath, "--inputs", inputs, "--materialization", material, "--profile", route.Profile}
	return exec.Command(binary, arguments...).CombinedOutput()
}
