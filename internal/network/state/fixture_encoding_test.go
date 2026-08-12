package state_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/epoch/assignment"
	"github.com/dianabuilds/ardents-network/internal/network/epoch/merkle"
)

type testEpochSpec struct {
	networkID             [32]byte
	number                uint64
	previous              [32]byte
	validFrom, validUntil time.Time
	inputs                [][]byte
	accepted              []fixtureRecord
	rejections            map[uint32]uint16
	assignmentSeed        [32]byte
	domains               []string
	authorities           []ed25519.PrivateKey
}

type testEpoch struct {
	Raw       []byte
	Digest    [32]byte
	Materials [][]byte
}

type testDomainSummary struct {
	id       string
	count    uint16
	capacity uint32
}

func buildTestEpoch(t *testing.T, spec testEpochSpec) testEpoch {
	t.Helper()
	accepted := append([]fixtureRecord(nil), spec.accepted...)
	sort.Slice(accepted, func(i, j int) bool {
		return bytes.Compare(accepted[i].nodeID[:], accepted[j].nodeID[:]) < 0
	})
	view := make([][]byte, len(accepted))
	for index := range accepted {
		view[index] = accepted[index].bytes
	}
	rejected := testRejectionLeaves(t, spec)
	summaries, familyCount, capacity, maxCount, maxCapacity := testSummaries(t, spec, accepted)
	unsigned := encodeTestEpoch(spec, view, rejected, summaries, familyCount, capacity, maxCount, maxCapacity)
	digest := sha256.Sum256(unsigned)
	raw := signTestEpoch(digest, unsigned, spec.authorities)
	materials := make([][]byte, len(view))
	for index, record := range view {
		materials[index] = encodeTestMaterial(digest, uint32(index), record, merkle.Proof(view, index, 0x11))
	}
	return testEpoch{Raw: raw, Digest: digest, Materials: materials}
}

func buildTestRecord(t *testing.T, network, nodeID [32]byte, private ed25519.PrivateKey,
	family, endpoint string, capacity uint16, from, until time.Time) fixtureRecord {
	t.Helper()
	buffer := new(bytes.Buffer)
	buffer.WriteString("ARNR")
	buffer.WriteByte(1)
	buffer.Write(network[:])
	buffer.Write(nodeID[:])
	writeTestU64(buffer, 1)
	writeTestI64(buffer, from.Unix())
	writeTestI64(buffer, until.Unix())
	writeTestText(buffer, family)
	buffer.WriteByte(1)
	writeTestText(buffer, endpoint)
	writeTestU16(buffer, capacity)
	buffer.Write(private.Public().(ed25519.PublicKey))
	buffer.Write(ed25519.Sign(private, buffer.Bytes()))
	return fixtureRecord{bytes: buffer.Bytes(), nodeID: nodeID, family: family, capacity: capacity}
}

func testRejectionLeaves(t *testing.T, spec testEpochSpec) [][32]byte {
	t.Helper()
	indices := make([]int, 0, len(spec.rejections))
	for index := range spec.rejections {
		indices = append(indices, int(index))
	}
	sort.Ints(indices)
	values := make([][32]byte, 0, len(indices))
	for _, index := range indices {
		if index < 0 || index >= len(spec.inputs) {
			t.Fatalf("rejection index %d is outside fixture inputs", index)
		}
		values = append(values, merkle.RejectionLeaf(uint32(index), spec.rejections[uint32(index)], spec.inputs[index]))
	}
	return values
}

func testSummaries(t *testing.T, spec testEpochSpec, records []fixtureRecord) ([]testDomainSummary, uint16, uint32, uint16, uint32) {
	t.Helper()
	values := make([]testDomainSummary, len(spec.domains))
	for index, domain := range spec.domains {
		values[index].id = domain
	}
	families := make(map[string][2]uint32)
	var capacity uint32
	for _, record := range records {
		selected, err := assignment.Select(spec.networkID, spec.number, spec.assignmentSeed, record.family, spec.domains)
		if err != nil {
			t.Fatal(err)
		}
		for index := range values {
			if values[index].id == selected {
				values[index].count++
				values[index].capacity += uint32(record.capacity)
			}
		}
		family := families[record.family]
		family[0]++
		family[1] += uint32(record.capacity)
		families[record.family] = family
		capacity += uint32(record.capacity)
	}
	var maxCount, maxCapacity uint32
	for _, family := range families {
		maxCount = max(maxCount, family[0])
		maxCapacity = max(maxCapacity, family[1])
	}
	return values, uint16(len(families)), capacity, uint16(maxCount), maxCapacity
}

func encodeTestEpoch(spec testEpochSpec, view [][]byte, rejected [][32]byte, summaries []testDomainSummary,
	families uint16, capacity uint32, maxFamilyCount uint16, maxFamilyCapacity uint32) []byte {
	buffer := new(bytes.Buffer)
	buffer.WriteString("AREP")
	buffer.WriteByte(1)
	buffer.Write(spec.networkID[:])
	writeTestU64(buffer, spec.number)
	buffer.Write(spec.previous[:])
	writeTestI64(buffer, spec.validFrom.Unix())
	writeTestI64(buffer, spec.validUntil.Unix())
	writeTestU32(buffer, uint32(len(spec.inputs)))
	writeTestText(buffer, "h3-role-probe-v1")
	inputRoot, viewRoot := merkle.Root(spec.inputs, 0x10), merkle.Root(view, 0x11)
	buffer.Write(inputRoot[:])
	buffer.Write(viewRoot[:])
	writeTestU32(buffer, uint32(len(view)))
	rejectedRoot := merkle.HashedRoot(rejected, 0x12)
	buffer.Write(rejectedRoot[:])
	writeTestU32(buffer, uint32(len(rejected)))
	buffer.Write(spec.assignmentSeed[:])
	writeTestText(buffer, "ardents-h3-role-domain-v1")
	writeTestU32(buffer, uint32(len(view)))
	writeTestU32(buffer, capacity)
	writeTestU16(buffer, families)
	writeTestU16(buffer, maxFamilyCount)
	writeTestU32(buffer, maxFamilyCapacity)
	buffer.WriteByte(byte(len(summaries)))
	for _, value := range summaries {
		writeTestText(buffer, value.id)
		writeTestU16(buffer, value.count)
		writeTestU32(buffer, value.capacity)
	}
	return buffer.Bytes()
}

func signTestEpoch(digest [32]byte, unsigned []byte, authorities []ed25519.PrivateKey) []byte {
	type signature struct {
		id  [32]byte
		raw []byte
	}
	values := make([]signature, len(authorities))
	for index, private := range authorities {
		public := private.Public().(ed25519.PublicKey)
		values[index] = signature{sha256.Sum256(public), ed25519.Sign(private, digest[:])}
	}
	sort.Slice(values, func(i, j int) bool { return bytes.Compare(values[i].id[:], values[j].id[:]) < 0 })
	raw := append([]byte(nil), unsigned...)
	raw = append(raw, byte(len(values)))
	for _, value := range values {
		raw = append(raw, value.id[:]...)
		raw = append(raw, value.raw...)
	}
	return raw
}

func encodeTestMaterial(digest [32]byte, index uint32, record []byte, siblings [][32]byte) []byte {
	buffer := new(bytes.Buffer)
	buffer.Write(digest[:])
	writeTestU32(buffer, index)
	writeTestU32(buffer, uint32(len(record)))
	buffer.Write(record)
	writeTestU16(buffer, uint16(len(siblings)))
	for _, sibling := range siblings {
		buffer.Write(sibling[:])
	}
	return buffer.Bytes()
}

func writeTestText(buffer *bytes.Buffer, value string) {
	buffer.WriteByte(byte(len(value)))
	buffer.WriteString(value)
}

func writeTestU16(buffer *bytes.Buffer, value uint16) {
	var raw [2]byte
	binary.BigEndian.PutUint16(raw[:], value)
	buffer.Write(raw[:])
}

func writeTestU32(buffer *bytes.Buffer, value uint32) {
	var raw [4]byte
	binary.BigEndian.PutUint32(raw[:], value)
	buffer.Write(raw[:])
}

func writeTestU64(buffer *bytes.Buffer, value uint64) {
	var raw [8]byte
	binary.BigEndian.PutUint64(raw[:], value)
	buffer.Write(raw[:])
}

func writeTestI64(buffer *bytes.Buffer, value int64) { writeTestU64(buffer, uint64(value)) }
