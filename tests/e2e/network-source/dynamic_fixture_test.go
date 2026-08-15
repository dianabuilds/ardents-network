package state_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	epochfixture "github.com/dianabuilds/ardents-network/tests/fixtures/networkfixture"
)

type verifierFixture struct {
	root             string
	now              int64
	networkID        [32]byte
	authorityID      [32]byte
	authorityPublic  ed25519.PublicKey
	authorityPrivate ed25519.PrivateKey
	generation       string
	epoch            uint64
	materializations [][]byte
	inputs           [][]byte
	accepted         []epochfixture.Record
	rejections       map[uint32]uint16
}

func writeVerifierFixtureAt(t *testing.T, unix int64) verifierFixture {
	t.Helper()
	now := time.Unix(unix, 0).UTC()
	network := sha256.Sum256([]byte("ardents-h3-stage-1-network"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xa9}, ed25519.SeedSize))
	first := verifierNodeRecord(t, network, 0x21, "family-b", "127.0.0.1:4102", 3, now.Add(-time.Minute), now.Add(time.Hour))
	second := verifierNodeRecord(t, network, 0x11, "family-a", "127.0.0.1:4101", 5, now.Add(-time.Minute), now.Add(time.Hour))
	rejected := verifierNodeRecord(t, network, 0x31, "family-c", "127.0.0.1:4103", 0, now.Add(-time.Minute), now.Add(time.Hour))
	duplicate := verifierNodeRecord(t, network, 0x41, "family-d", "127.0.0.1:4104", 2, now.Add(-time.Minute), now.Add(time.Hour))
	sourceCollision := verifierRecordWithKey(t, network, sha256.Sum256([]byte("source-collision-node")), authority,
		"family-e", "127.0.0.1:4105", 2, now.Add(-time.Minute), now.Add(time.Hour))
	future := verifierNodeRecord(t, network, 0x51, "family-f", "127.0.0.1:4106", 2, now.Add(time.Minute), now.Add(time.Hour))
	inputs := [][]byte{first.Raw, rejected.Raw, second.Raw, []byte("malformed"), duplicate.Raw, duplicate.Raw, sourceCollision.Raw, future.Raw}
	accepted := []epochfixture.Record{first, second}
	sort.Slice(accepted, func(i, j int) bool { return bytes.Compare(accepted[i].NodeID[:], accepted[j].NodeID[:]) < 0 })
	rejections := map[uint32]uint16{1: 6, 3: 1, 4: 8, 5: 8, 6: 7, 7: 4}
	built := buildVerifierEpoch(t, network, 1, [32]byte{}, now, inputs, accepted, rejections, authority)
	fixture := verifierFixture{now: unix, networkID: network, authorityID: sha256.Sum256(authority.Public().(ed25519.PublicKey)),
		authorityPublic: authority.Public().(ed25519.PublicKey), authorityPrivate: authority, generation: fmt.Sprintf("%x", built.Digest), epoch: built.Number,
		materializations: [][]byte{built.Materials[0]}, inputs: inputs, accepted: accepted, rejections: rejections}
	fixture.root = writeVerifierRoot(t, fixture.generation, built.Raw, inputs)
	return fixture
}

func writeVerifierSuccessor(t *testing.T, previous verifierFixture) verifierFixture {
	t.Helper()
	now := time.Unix(previous.now, 0).UTC()
	prior, err := os.ReadFile(filepath.Join(previous.root, "generations", previous.generation, "epoch.bin"))
	if err != nil {
		t.Fatal(err)
	}
	unsignedEnd := len(prior) - 1 - 32 - ed25519.SignatureSize
	digest := sha256.Sum256(prior[:unsignedEnd])
	built := buildVerifierEpoch(t, previous.networkID, 2, digest, now, previous.inputs, previous.accepted, previous.rejections, previous.authorityPrivate)
	fixture := previous
	fixture.generation = fmt.Sprintf("%x", built.Digest)
	fixture.epoch = built.Number
	fixture.materializations = [][]byte{built.Materials[0]}
	fixture.root = writeVerifierRoot(t, fixture.generation, built.Raw, previous.inputs)
	return fixture
}

func buildVerifierEpoch(t *testing.T, network [32]byte, number uint64, previous [32]byte, now time.Time, inputs [][]byte,
	accepted []epochfixture.Record, rejections map[uint32]uint16, authority ed25519.PrivateKey) epochfixture.Epoch {
	t.Helper()
	seed := sha256.Sum256([]byte("assignment-seed-1"))
	built, err := epochfixture.BuildEpoch(epochfixture.EpochSpec{NetworkID: network, Number: number, Previous: previous,
		ValidFrom: now.Add(-30 * time.Second), ValidUntil: now.Add(30 * time.Minute), Inputs: inputs, Accepted: accepted,
		Rejections: rejections, AssignmentSeed: seed, Domains: []string{"alpha", "beta"}, Authorities: []ed25519.PrivateKey{authority}})
	if err != nil {
		t.Fatal(err)
	}
	return built
}

func verifierNodeRecord(t *testing.T, network [32]byte, marker byte, family, endpoint string, capacity uint16, from, until time.Time) epochfixture.Record {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	nodeID := sha256.Sum256([]byte{0x4e, marker})
	return verifierRecordWithKey(t, network, nodeID, private, family, endpoint, capacity, from, until)
}

func verifierRecordWithKey(t *testing.T, network, nodeID [32]byte, private ed25519.PrivateKey, family, endpoint string, capacity uint16, from, until time.Time) epochfixture.Record {
	t.Helper()
	record, err := epochfixture.BuildRecord(epochfixture.RecordSpec{NetworkID: network, NodeID: nodeID, Generation: 1,
		ValidFrom: from, ValidUntil: until, Family: family, Endpoint: endpoint, Capability: 1, Capacity: capacity, PrivateKey: private})
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func writeVerifierRoot(t *testing.T, generation string, epoch []byte, inputs [][]byte) string {
	t.Helper()
	root := t.TempDir()
	directory := filepath.Join(root, "generations", generation, "inputs")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "current"), []byte(generation+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "generations", generation, "epoch.bin"), epoch, 0o600); err != nil {
		t.Fatal(err)
	}
	for index, input := range inputs {
		if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("%04d.bin", index)), input, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
