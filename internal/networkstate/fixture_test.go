package networkstate_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/networkstate"
)

const (
	fixtureNow       = int64(1_800_000_100)
	rejectCapacity   = uint16(6)
	roleProbeProfile = "h3-role-probe-v1"
	assignmentV1     = "ardents-h3-role-domain-v1"
)

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
	materializations []networkstate.Materialization
}

type fixtureRecord struct {
	bytes    []byte
	nodeID   [32]byte
	family   string
	capacity uint16
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	networkID := sha256.Sum256([]byte("ardents-h3-stage-1-network"))
	authorityPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xa9}, ed25519.SeedSize))
	authorityPublic := authorityPrivate.Public().(ed25519.PublicKey)
	authorityID := sha256.Sum256(authorityPublic)

	first := makeRecord(t, networkID, 0x21, "family-b", "127.0.0.1:4102", 3)
	second := makeRecord(t, networkID, 0x11, "family-a", "127.0.0.1:4101", 5)
	rejected := makeRecord(t, networkID, 0x31, "family-c", "127.0.0.1:4103", 0)
	duplicate := makeRecord(t, networkID, 0x41, "family-d", "127.0.0.1:4104", 2)
	sourceCollision := makeRecordWithKey(
		networkID, sha256.Sum256([]byte("source-collision-node")), authorityPrivate,
		"family-e", "127.0.0.1:4105", 2, fixtureNow-60, fixtureNow+3600,
	)
	future := makeRecordAt(t, networkID, 0x51, "family-f", "127.0.0.1:4106", 2, fixtureNow+60, fixtureNow+3600)
	inputs := [][]byte{
		first.bytes, rejected.bytes, second.bytes, []byte("malformed"),
		duplicate.bytes, duplicate.bytes, sourceCollision.bytes, future.bytes,
	}
	accepted := []fixtureRecord{first, second}
	sort.Slice(accepted, func(i, j int) bool {
		return bytes.Compare(accepted[i].nodeID[:], accepted[j].nodeID[:]) < 0
	})
	viewBytes := [][]byte{accepted[0].bytes, accepted[1].bytes}
	inputRoot := merkleRoot(inputs, 0x10)
	viewRoot := merkleRoot(viewBytes, 0x11)
	rejectionRoot := hashedMerkleRoot([][32]byte{
		rejectionLeaf(1, rejectCapacity, rejected.bytes),
		rejectionLeaf(3, 1, inputs[3]),
		rejectionLeaf(4, 8, duplicate.bytes),
		rejectionLeaf(5, 8, duplicate.bytes),
		rejectionLeaf(6, 7, sourceCollision.bytes),
		rejectionLeaf(7, 4, future.bytes),
	}, 0x12)

	unsigned := new(bytes.Buffer)
	unsigned.WriteString("AREP")
	unsigned.WriteByte(1)
	unsigned.Write(networkID[:])
	writeU64(unsigned, 1)
	unsigned.Write(make([]byte, 32))
	writeI64(unsigned, fixtureNow-30)
	writeI64(unsigned, fixtureNow+1800)
	writeU32(unsigned, uint32(len(inputs)))
	writeText(unsigned, roleProbeProfile)
	unsigned.Write(inputRoot[:])
	unsigned.Write(viewRoot[:])
	writeU32(unsigned, uint32(len(viewBytes)))
	unsigned.Write(rejectionRoot[:])
	writeU32(unsigned, 6)
	seed := sha256.Sum256([]byte("assignment-seed-1"))
	unsigned.Write(seed[:])
	writeText(unsigned, assignmentV1)
	writeU32(unsigned, 2)
	writeU32(unsigned, 8)
	writeU16(unsigned, 2)
	writeU16(unsigned, 1)
	writeU32(unsigned, 5)
	unsigned.WriteByte(2)
	for _, summary := range fixtureDomainSummaries(networkID, 1, seed, accepted) {
		writeText(unsigned, summary.id)
		writeU16(unsigned, summary.count)
		writeU32(unsigned, summary.capacity)
	}

	digest := sha256.Sum256(unsigned.Bytes())
	epoch := append([]byte(nil), unsigned.Bytes()...)
	epoch = append(epoch, 1)
	epoch = append(epoch, authorityID[:]...)
	epoch = append(epoch, ed25519.Sign(authorityPrivate, digest[:])...)

	materials := []networkstate.Materialization{{
		EpochDigest: digest,
		Index:       0,
		Record:      append([]byte(nil), viewBytes[0]...),
		Siblings:    merkleProof(viewBytes, 0),
	}}
	return fixture{
		now:              fixtureNow,
		networkID:        networkID,
		authorityID:      authorityID,
		authorityPublic:  authorityPublic,
		authorityPrivate: authorityPrivate,
		epoch:            epoch,
		epochDigest:      digest,
		inputs:           inputs,
		inputRoot:        inputRoot,
		viewRoot:         viewRoot,
		rejectedRoot:     rejectionRoot,
		accepted:         accepted,
		materializations: materials,
	}
}

func nextFixture(t *testing.T, previous fixture) fixture {
	t.Helper()
	seed := sha256.Sum256([]byte("assignment-seed-2"))
	unsigned := new(bytes.Buffer)
	unsigned.WriteString("AREP")
	unsigned.WriteByte(1)
	unsigned.Write(previous.networkID[:])
	writeU64(unsigned, 2)
	unsigned.Write(previous.epochDigest[:])
	writeI64(unsigned, fixtureNow-30)
	writeI64(unsigned, fixtureNow+3600)
	writeU32(unsigned, uint32(len(previous.inputs)))
	writeText(unsigned, roleProbeProfile)
	unsigned.Write(previous.inputRoot[:])
	unsigned.Write(previous.viewRoot[:])
	writeU32(unsigned, uint32(len(previous.accepted)))
	unsigned.Write(previous.rejectedRoot[:])
	writeU32(unsigned, 6)
	unsigned.Write(seed[:])
	writeText(unsigned, assignmentV1)
	writeU32(unsigned, 2)
	writeU32(unsigned, 8)
	writeU16(unsigned, 2)
	writeU16(unsigned, 1)
	writeU32(unsigned, 5)
	unsigned.WriteByte(2)
	for _, summary := range fixtureDomainSummaries(previous.networkID, 2, seed, previous.accepted) {
		writeText(unsigned, summary.id)
		writeU16(unsigned, summary.count)
		writeU32(unsigned, summary.capacity)
	}
	digest := sha256.Sum256(unsigned.Bytes())
	epoch := append([]byte(nil), unsigned.Bytes()...)
	epoch = append(epoch, 1)
	epoch = append(epoch, previous.authorityID[:]...)
	epoch = append(epoch, ed25519.Sign(previous.authorityPrivate, digest[:])...)
	view := [][]byte{previous.accepted[0].bytes, previous.accepted[1].bytes}
	previous.epoch, previous.epochDigest = epoch, digest
	previous.materializations = []networkstate.Materialization{{
		EpochDigest: digest, Index: 0, Record: append([]byte(nil), view[0]...), Siblings: merkleProof(view, 0),
	}}
	return previous
}

type fixtureDomainSummary struct {
	id       string
	count    uint16
	capacity uint32
}

func fixtureDomainSummaries(network [32]byte, epoch uint64, seed [32]byte, accepted []fixtureRecord) []fixtureDomainSummary {
	domains := []fixtureDomainSummary{{id: "alpha"}, {id: "beta"}}
	for _, record := range accepted {
		selected := 0
		first := fixtureAssignmentDigest(network, epoch, seed, record.family, domains[0].id)
		second := fixtureAssignmentDigest(network, epoch, seed, record.family, domains[1].id)
		if bytes.Compare(second[:], first[:]) < 0 {
			selected = 1
		}
		domains[selected].count++
		domains[selected].capacity += uint32(record.capacity)
	}
	return domains
}

func fixtureAssignmentDigest(network [32]byte, epoch uint64, seed [32]byte, family, domain string) [32]byte {
	buf := new(bytes.Buffer)
	buf.WriteString("ardents-h3-role-domain-v1\x00")
	buf.Write(network[:])
	writeU64(buf, epoch)
	buf.Write(seed[:])
	buf.WriteString(family)
	buf.WriteString(domain)
	return sha256.Sum256(buf.Bytes())
}

func makeRecord(t *testing.T, networkID [32]byte, marker byte, family, endpoint string, capacity uint16) fixtureRecord {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	nodeID := sha256.Sum256([]byte{0x4e, marker})
	return makeRecordWithKey(networkID, nodeID, private, family, endpoint, capacity, fixtureNow-60, fixtureNow+3600)
}

func makeRecordAt(t *testing.T, networkID [32]byte, marker byte, family, endpoint string, capacity uint16, notBefore, notAfter int64) fixtureRecord {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	nodeID := sha256.Sum256([]byte{0x4e, marker})
	return makeRecordWithKey(networkID, nodeID, private, family, endpoint, capacity, notBefore, notAfter)
}

func makeRecordWithKey(networkID, nodeID [32]byte, private ed25519.PrivateKey, family, endpoint string, capacity uint16, notBefore, notAfter int64) fixtureRecord {
	public := private.Public().(ed25519.PublicKey)
	buf := new(bytes.Buffer)
	buf.WriteString("ARNR")
	buf.WriteByte(1)
	buf.Write(networkID[:])
	buf.Write(nodeID[:])
	writeU64(buf, 1)
	writeI64(buf, notBefore)
	writeI64(buf, notAfter)
	writeText(buf, family)
	buf.WriteByte(1)
	writeText(buf, endpoint)
	writeU16(buf, capacity)
	buf.Write(public)
	buf.Write(ed25519.Sign(private, buf.Bytes()))
	return fixtureRecord{bytes: buf.Bytes(), nodeID: nodeID, family: family, capacity: capacity}
}

func merkleRoot(values [][]byte, emptyTag byte) [32]byte {
	if len(values) == 0 {
		return sha256.Sum256([]byte{emptyTag})
	}
	if len(values) == 1 {
		return recordLeaf(values[0])
	}
	split := merkleSplit(len(values))
	left := merkleRoot(values[:split], emptyTag)
	right := merkleRoot(values[split:], emptyTag)
	return branchHash(left, right)
}

func recordLeaf(value []byte) [32]byte {
	buf := make([]byte, 5+len(value))
	buf[0] = 0
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(value)))
	copy(buf[5:], value)
	return sha256.Sum256(buf)
}

func rejectionLeaf(index uint32, code uint16, raw []byte) [32]byte {
	rawDigest := sha256.Sum256(raw)
	buf := make([]byte, 1+4+2+32)
	buf[0] = 2
	binary.BigEndian.PutUint32(buf[1:5], index)
	binary.BigEndian.PutUint16(buf[5:7], code)
	copy(buf[7:], rawDigest[:])
	return sha256.Sum256(buf)
}

func hashedMerkleRoot(leaves [][32]byte, emptyTag byte) [32]byte {
	if len(leaves) == 0 {
		return sha256.Sum256([]byte{emptyTag})
	}
	if len(leaves) == 1 {
		return leaves[0]
	}
	split := merkleSplit(len(leaves))
	return branchHash(
		hashedMerkleRoot(leaves[:split], emptyTag),
		hashedMerkleRoot(leaves[split:], emptyTag),
	)
}

func branchHash(left, right [32]byte) [32]byte {
	buf := make([]byte, 65)
	buf[0] = 1
	copy(buf[1:33], left[:])
	copy(buf[33:], right[:])
	return sha256.Sum256(buf)
}

func merkleSplit(length int) int {
	split := 1
	for split<<1 < length {
		split <<= 1
	}
	return split
}

func merkleProof(values [][]byte, index int) [][32]byte {
	if len(values) <= 1 {
		return nil
	}
	split := merkleSplit(len(values))
	if index < split {
		proof := merkleProof(values[:split], index)
		return append(proof, merkleRoot(values[split:], 0x11))
	}
	proof := merkleProof(values[split:], index-split)
	return append(proof, merkleRoot(values[:split], 0x11))
}

func writeText(buf *bytes.Buffer, value string) {
	buf.WriteByte(byte(len(value)))
	buf.WriteString(value)
}

func writeU16(buf *bytes.Buffer, value uint16) {
	var encoded [2]byte
	binary.BigEndian.PutUint16(encoded[:], value)
	buf.Write(encoded[:])
}

func writeU32(buf *bytes.Buffer, value uint32) {
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], value)
	buf.Write(encoded[:])
}

func writeU64(buf *bytes.Buffer, value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	buf.Write(encoded[:])
}

func writeI64(buf *bytes.Buffer, value int64) {
	writeU64(buf, uint64(value))
}
