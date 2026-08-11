package qualification_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"time"
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
	number    uint64
	seed      [32]byte
	raw       []byte
	digest    [32]byte
	inputs    [][]byte
	materials [][]byte
}

type lifecycleRecord struct {
	raw      []byte
	nodeID   [32]byte
	private  ed25519.PrivateKey
	family   string
	endpoint string
	capacity uint16
}

func newLifecycleStateFixture(endpoints [2]string) lifecycleStateFixture {
	now := time.Now().Unix()
	network := sha256.Sum256([]byte("ardents-h3-s1-lifecycle-network"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xa9}, ed25519.SeedSize))
	records := []lifecycleRecord{
		makeLifecycleRecord(network, 0x11, "family-a", endpoints[0], now),
		makeLifecycleRecord(network, 0x21, "family-b", endpoints[1], now),
	}
	sort.Slice(records, func(i, j int) bool { return bytes.Compare(records[i].nodeID[:], records[j].nodeID[:]) < 0 })
	fixture := lifecycleStateFixture{now: now, network: network, authorityPublic: authority.Public().(ed25519.PublicKey),
		authorityPrivate: authority, authorityID: sha256.Sum256(authority.Public().(ed25519.PublicKey)), records: records}
	firstSeed := sha256.Sum256([]byte("lifecycle-assignment-one"))
	fixture.genesis = fixture.makeEpoch(1, [32]byte{}, firstSeed)
	for marker := byte(1); ; marker++ {
		seed := sha256.Sum256([]byte{marker, 0x52})
		candidate := fixture.makeEpoch(2, fixture.genesis.digest, seed)
		if assignmentsChanged(fixture, firstSeed, seed) {
			fixture.successor = candidate
			break
		}
	}
	return fixture
}

func makeLifecycleRecord(network [32]byte, marker byte, family, endpoint string, now int64) lifecycleRecord {
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	nodeID := sha256.Sum256([]byte{0x4e, marker})
	buffer := new(bytes.Buffer)
	buffer.WriteString("ARNR")
	buffer.WriteByte(1)
	buffer.Write(network[:])
	buffer.Write(nodeID[:])
	writeU64(buffer, 1)
	writeI64(buffer, now-60)
	writeI64(buffer, now+3600)
	writeText(buffer, family)
	buffer.WriteByte(1)
	writeText(buffer, endpoint)
	writeU16(buffer, 4)
	buffer.Write(private.Public().(ed25519.PublicKey))
	buffer.Write(ed25519.Sign(private, buffer.Bytes()))
	return lifecycleRecord{raw: buffer.Bytes(), nodeID: nodeID, private: private, family: family, endpoint: endpoint, capacity: 4}
}

func (fixture lifecycleStateFixture) makeEpoch(number uint64, previous, seed [32]byte) lifecycleEpoch {
	inputs := make([][]byte, len(fixture.records))
	for index := range fixture.records {
		inputs[index] = fixture.records[index].raw
	}
	inputRoot, viewRoot := lifecycleMerkle(inputs, 0x10), lifecycleMerkle(inputs, 0x11)
	buffer := new(bytes.Buffer)
	buffer.WriteString("AREP")
	buffer.WriteByte(1)
	buffer.Write(fixture.network[:])
	writeU64(buffer, number)
	buffer.Write(previous[:])
	writeI64(buffer, fixture.now-30)
	writeI64(buffer, fixture.now+1800)
	writeU32(buffer, uint32(len(inputs)))
	writeText(buffer, "h3-role-probe-v1")
	buffer.Write(inputRoot[:])
	buffer.Write(viewRoot[:])
	writeU32(buffer, uint32(len(inputs)))
	rejected := sha256.Sum256([]byte{0x12})
	buffer.Write(rejected[:])
	writeU32(buffer, 0)
	buffer.Write(seed[:])
	writeText(buffer, "ardents-h3-role-domain-v1")
	writeU32(buffer, 2)
	writeU32(buffer, 8)
	writeU16(buffer, 2)
	writeU16(buffer, 1)
	writeU32(buffer, 4)
	buffer.WriteByte(2)
	for _, summary := range lifecycleSummaries(fixture.network, number, seed, fixture.records) {
		writeText(buffer, summary.domain)
		writeU16(buffer, summary.count)
		writeU32(buffer, summary.capacity)
	}
	digest := sha256.Sum256(buffer.Bytes())
	raw := append([]byte(nil), buffer.Bytes()...)
	raw = append(raw, 1)
	raw = append(raw, fixture.authorityID[:]...)
	raw = append(raw, ed25519.Sign(fixture.authorityPrivate, digest[:])...)
	materials := make([][]byte, len(inputs))
	for index, record := range inputs {
		proof := [][32]byte{lifecycleLeaf(inputs[1-index])}
		materials[index] = lifecycleMaterial(digest, uint32(index), record, proof)
	}
	return lifecycleEpoch{number: number, seed: seed, raw: raw, digest: digest, inputs: inputs, materials: materials}
}

type lifecycleSummary struct {
	domain   string
	count    uint16
	capacity uint32
}

func lifecycleSummaries(network [32]byte, epoch uint64, seed [32]byte, records []lifecycleRecord) []lifecycleSummary {
	values := []lifecycleSummary{{domain: "alpha"}, {domain: "beta"}}
	for _, record := range records {
		index := 0
		alpha := lifecycleAssignment(network, epoch, seed, record.family, "alpha")
		beta := lifecycleAssignment(network, epoch, seed, record.family, "beta")
		if bytes.Compare(beta[:], alpha[:]) < 0 {
			index = 1
		}
		values[index].count++
		values[index].capacity += uint32(record.capacity)
	}
	return values
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
	alpha := lifecycleAssignment(network, epoch, seed, family, "alpha")
	beta := lifecycleAssignment(network, epoch, seed, family, "beta")
	if bytes.Compare(beta[:], alpha[:]) < 0 {
		return "beta"
	}
	return "alpha"
}

func lifecycleAssignment(network [32]byte, epoch uint64, seed [32]byte, family, domain string) [32]byte {
	buffer := new(bytes.Buffer)
	buffer.WriteString("ardents-h3-role-domain-v1\x00")
	buffer.Write(network[:])
	writeU64(buffer, epoch)
	buffer.Write(seed[:])
	buffer.WriteString(family)
	buffer.WriteString(domain)
	return sha256.Sum256(buffer.Bytes())
}

func lifecycleLeaf(value []byte) [32]byte {
	encoded := make([]byte, 5+len(value))
	binary.BigEndian.PutUint32(encoded[1:5], uint32(len(value)))
	copy(encoded[5:], value)
	return sha256.Sum256(encoded)
}

func lifecycleMerkle(values [][]byte, empty byte) [32]byte {
	if len(values) == 0 {
		return sha256.Sum256([]byte{empty})
	}
	if len(values) == 1 {
		return lifecycleLeaf(values[0])
	}
	left, right := lifecycleLeaf(values[0]), lifecycleLeaf(values[1])
	return sha256.Sum256(append(append([]byte{1}, left[:]...), right[:]...))
}

func lifecycleMaterial(digest [32]byte, index uint32, record []byte, proof [][32]byte) []byte {
	buffer := new(bytes.Buffer)
	buffer.Write(digest[:])
	writeU32(buffer, index)
	writeU32(buffer, uint32(len(record)))
	buffer.Write(record)
	writeU16(buffer, uint16(len(proof)))
	for _, sibling := range proof {
		buffer.Write(sibling[:])
	}
	return buffer.Bytes()
}

func writeText(buffer *bytes.Buffer, value string) {
	buffer.WriteByte(byte(len(value)))
	buffer.WriteString(value)
}
func writeU16(buffer *bytes.Buffer, value uint16) {
	var raw [2]byte
	binary.BigEndian.PutUint16(raw[:], value)
	buffer.Write(raw[:])
}
func writeU32(buffer *bytes.Buffer, value uint32) {
	var raw [4]byte
	binary.BigEndian.PutUint32(raw[:], value)
	buffer.Write(raw[:])
}
func writeU64(buffer *bytes.Buffer, value uint64) {
	var raw [8]byte
	binary.BigEndian.PutUint64(raw[:], value)
	buffer.Write(raw[:])
}
func writeI64(buffer *bytes.Buffer, value int64) { writeU64(buffer, uint64(value)) }
