package state_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"sort"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/epoch/assignment"
)

type lifecycleStateFixture struct {
	now              int64
	network          [32]byte
	authorityPublic  ed25519.PublicKey
	authorityPrivate ed25519.PrivateKey
	authorityID      [32]byte
	records          []lifecycleRecord
	genesis          lifecycleEpoch
	successor        lifecycleEpoch
}

type lifecycleEpoch struct {
	number            uint64
	seed              [32]byte
	raw               []byte
	digest            [32]byte
	inputs, materials [][]byte
}
type lifecycleRecord struct {
	raw              []byte
	nodeID           [32]byte
	private          ed25519.PrivateKey
	family, endpoint string
	capacity         uint16
}

func newLifecycleStateFixture(t *testing.T, endpoints [2]string) lifecycleStateFixture {
	t.Helper()
	now := time.Now().UTC()
	network := sha256.Sum256([]byte("ardents-h3-node-network"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xa9}, ed25519.SeedSize))
	records := []lifecycleRecord{
		makeLifecycleRecord(t, network, 0x11, "family-a", endpoints[0], now),
		makeLifecycleRecord(t, network, 0x21, "family-b", endpoints[1], now),
	}
	sort.Slice(records, func(i, j int) bool { return bytes.Compare(records[i].nodeID[:], records[j].nodeID[:]) < 0 })
	fixture := lifecycleStateFixture{now: now.Unix(), network: network, authorityPublic: authority.Public().(ed25519.PublicKey),
		authorityPrivate: authority, authorityID: sha256.Sum256(authority.Public().(ed25519.PublicKey)), records: records}
	firstSeed := sha256.Sum256([]byte("lifecycle-assignment-one"))
	fixture.genesis = fixture.makeEpoch(t, 1, [32]byte{}, firstSeed)
	for marker := byte(1); ; marker++ {
		seed := sha256.Sum256([]byte{marker, 0x52})
		candidate := fixture.makeEpoch(t, 2, fixture.genesis.digest, seed)
		if assignmentsChanged(fixture, firstSeed, seed) {
			fixture.successor = candidate
			break
		}
	}
	return fixture
}

func makeLifecycleRecord(t *testing.T, network [32]byte, marker byte, family, endpoint string, now time.Time) lifecycleRecord {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	nodeID := sha256.Sum256([]byte{0x4e, marker})
	record, err := BuildRecord(RecordSpec{NetworkID: network, NodeID: nodeID, Generation: 1,
		ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour), Family: family, Endpoint: endpoint,
		Capability: 1, Capacity: 4, PrivateKey: private})
	if err != nil {
		t.Fatal(err)
	}
	return lifecycleRecord{raw: record.Raw, nodeID: nodeID, private: private, family: family, endpoint: endpoint, capacity: 4}
}

func (fixture lifecycleStateFixture) makeEpoch(t *testing.T, number uint64, previous, seed [32]byte) lifecycleEpoch {
	t.Helper()
	inputs := make([][]byte, len(fixture.records))
	accepted := make([]Record, len(fixture.records))
	for index, record := range fixture.records {
		inputs[index] = record.raw
		accepted[index] = Record{Raw: record.raw, NodeID: record.nodeID, Family: record.family, Capacity: record.capacity}
	}
	now := time.Unix(fixture.now, 0).UTC()
	built, err := BuildEpoch(EpochSpec{NetworkID: fixture.network, Number: number, Previous: previous,
		ValidFrom: now.Add(-30 * time.Second), ValidUntil: now.Add(30 * time.Minute), Inputs: inputs, Accepted: accepted,
		AssignmentSeed: seed, Domains: []string{"alpha", "beta"}, Authorities: []ed25519.PrivateKey{fixture.authorityPrivate}})
	if err != nil {
		t.Fatal(err)
	}
	return lifecycleEpoch{number: number, seed: seed, raw: built.Raw, digest: built.Digest, inputs: built.Inputs, materials: built.Materials}
}

func assignmentsChanged(fixture lifecycleStateFixture, first, second [32]byte) bool {
	for _, record := range fixture.records {
		if selectedDomain(fixture.network, 1, first, record.family) != selectedDomain(fixture.network, 2, second, record.family) {
			return true
		}
	}
	return false
}

func selectedDomain(network [32]byte, epoch uint64, seed [32]byte, family string) string {
	selected, _ := assignment.Select(network, epoch, seed, family, []string{"alpha", "beta"})
	return selected
}
