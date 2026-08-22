package state_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"sort"
	"testing"
	"time"
)

const fixtureNow = int64(1_800_000_100)

type fixture struct {
	now              int64
	networkID        [32]byte
	authorityID      [32]byte
	authorityPublic  ed25519.PublicKey
	authorityPrivate ed25519.PrivateKey
	epoch            []byte
	epochDigest      [32]byte
	inputs           [][]byte
	inputRoot        [32]byte
	viewRoot         [32]byte
	rejectedRoot     [32]byte
	accepted         []fixtureRecord
	rejections       map[uint32]uint16
	materializations [][]byte
}

type fixtureRecord struct {
	bytes    []byte
	nodeID   [32]byte
	family   string
	capacity uint16
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	now := time.Unix(fixtureNow, 0).UTC()
	network := sha256.Sum256([]byte("ardents-h3-stage-1-network"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xa9}, ed25519.SeedSize))
	first := makeRecord(t, network, 0x21, "family-b", "127.0.0.1:4102", 3)
	second := makeRecord(t, network, 0x11, "family-a", "127.0.0.1:4101", 5)
	rejected := makeRecord(t, network, 0x31, "family-c", "127.0.0.1:4103", 0)
	duplicate := makeRecord(t, network, 0x41, "family-d", "127.0.0.1:4104", 2)
	sourceCollision := makeRecordWithKey(t, network, sha256.Sum256([]byte("source-collision-node")), authority,
		"family-e", "127.0.0.1:4105", 2, fixtureNow-60, fixtureNow+3600)
	future := makeRecordAt(t, network, 0x51, "family-f", "127.0.0.1:4106", 2, fixtureNow+60, fixtureNow+3600)
	inputs := [][]byte{first.bytes, rejected.bytes, second.bytes, []byte("malformed"), duplicate.bytes, duplicate.bytes, sourceCollision.bytes, future.bytes}
	accepted := []fixtureRecord{first, second}
	sort.Slice(accepted, func(i, j int) bool { return bytes.Compare(accepted[i].nodeID[:], accepted[j].nodeID[:]) < 0 })
	value := fixture{now: fixtureNow, networkID: network, authorityID: sha256.Sum256(authority.Public().(ed25519.PublicKey)),
		authorityPublic: authority.Public().(ed25519.PublicKey), authorityPrivate: authority, inputs: inputs, accepted: accepted,
		rejections: map[uint32]uint16{1: 6, 3: 1, 4: 8, 5: 8, 6: 7, 7: 4}}
	return buildFixtureEpoch(t, value, 1, [32]byte{}, sha256.Sum256([]byte("assignment-seed-1")), now.Add(-30*time.Second), now.Add(30*time.Minute))
}

func nextFixture(t *testing.T, previous fixture) fixture {
	return nextFixtureWithSeed(t, previous, "assignment-seed-2")
}

func nextFixtureWithSeed(t *testing.T, previous fixture, label string) fixture {
	t.Helper()
	now := time.Unix(fixtureNow, 0).UTC()
	return buildFixtureEpoch(t, previous, 2, previous.epochDigest, sha256.Sum256([]byte(label)), now.Add(-30*time.Second), now.Add(time.Hour))
}

func futureFixture(t *testing.T, previous fixture, validFrom int64) fixture {
	t.Helper()
	value := buildFixtureEpoch(t, previous, 2, previous.epochDigest, sha256.Sum256([]byte("future-assignment-seed")),
		time.Unix(validFrom, 0).UTC(), time.Unix(fixtureNow+3600, 0).UTC())
	value.now = validFrom
	return value
}

func buildFixtureEpoch(t *testing.T, value fixture, number uint64, previous, seed [32]byte, from, until time.Time) fixture {
	t.Helper()
	built := buildTestEpoch(t, testEpochSpec{networkID: value.networkID, number: number, previous: previous,
		validFrom: from, validUntil: until, inputs: value.inputs, accepted: value.accepted, rejections: value.rejections,
		assignmentSeed: seed, domains: []string{"alpha", "beta"}, authorities: []ed25519.PrivateKey{value.authorityPrivate}})
	value.epoch, value.epochDigest, value.materializations = built.Raw, built.Digest, [][]byte{built.Materials[0]}
	view := make([][]byte, len(value.accepted))
	for index := range value.accepted {
		view[index] = value.accepted[index].bytes
	}
	rejected := make([][32]byte, 0, len(value.rejections))
	indices := make([]int, 0, len(value.rejections))
	for index := range value.rejections {
		indices = append(indices, int(index))
	}
	sort.Ints(indices)
	for _, index := range indices {
		rejected = append(rejected, fixtureRejectionLeaf(uint32(index), value.rejections[uint32(index)], value.inputs[index]))
	}
	value.inputRoot, value.viewRoot, value.rejectedRoot = fixtureCommitmentRoot(value.inputs, 0x10), fixtureCommitmentRoot(view, 0x11), fixtureHashedCommitmentRoot(rejected, 0x12)
	return value
}

func makeRecord(t *testing.T, network [32]byte, marker byte, family, endpoint string, capacity uint16) fixtureRecord {
	t.Helper()
	return makeRecordAt(t, network, marker, family, endpoint, capacity, fixtureNow-60, fixtureNow+3600)
}

func makeRecordAt(t *testing.T, network [32]byte, marker byte, family, endpoint string, capacity uint16, from, until int64) fixtureRecord {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	nodeID := sha256.Sum256([]byte{0x4e, marker})
	return makeRecordWithKey(t, network, nodeID, private, family, endpoint, capacity, from, until)
}

func makeRecordWithKey(t *testing.T, network, nodeID [32]byte, private ed25519.PrivateKey, family, endpoint string, capacity uint16, from, until int64) fixtureRecord {
	t.Helper()
	return buildTestRecord(t, network, nodeID, private, family, endpoint, capacity,
		time.Unix(from, 0).UTC(), time.Unix(until, 0).UTC())
}
