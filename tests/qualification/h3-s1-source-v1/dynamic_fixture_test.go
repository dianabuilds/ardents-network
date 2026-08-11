package qualification_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

type verifierFixture struct {
	root             string
	now              int64
	networkID        [32]byte
	authorityID      [32]byte
	authorityPublic  ed25519.PublicKey
	authorityPrivate ed25519.PrivateKey
	generation       string
	materializations [][]byte
}

type verifierRecord struct {
	raw      []byte
	nodeID   [32]byte
	family   string
	capacity uint16
}

func writeVerifierFixtureAt(t *testing.T, now int64) verifierFixture {
	t.Helper()
	networkID := sha256.Sum256([]byte("ardents-h3-stage-1-network"))
	authorityPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xa9}, ed25519.SeedSize))
	authorityPublic := authorityPrivate.Public().(ed25519.PublicKey)
	authorityID := sha256.Sum256(authorityPublic)
	first := verifierNodeRecord(networkID, 0x21, "family-b", "127.0.0.1:4102", 3, now)
	second := verifierNodeRecord(networkID, 0x11, "family-a", "127.0.0.1:4101", 5, now)
	rejected := verifierNodeRecord(networkID, 0x31, "family-c", "127.0.0.1:4103", 0, now)
	duplicate := verifierNodeRecord(networkID, 0x41, "family-d", "127.0.0.1:4104", 2, now)
	sourceCollision := verifierNodeRecordWithKey(
		networkID, sha256.Sum256([]byte("source-collision-node")), authorityPrivate,
		"family-e", "127.0.0.1:4105", 2, now-60, now+3600,
	)
	future := verifierNodeRecordAt(networkID, 0x51, "family-f", "127.0.0.1:4106", 2, now+60, now+3600)
	inputs := [][]byte{
		first.raw, rejected.raw, second.raw, []byte("malformed"),
		duplicate.raw, duplicate.raw, sourceCollision.raw, future.raw,
	}
	accepted := []verifierRecord{first, second}
	sort.Slice(accepted, func(i, j int) bool {
		return bytes.Compare(accepted[i].nodeID[:], accepted[j].nodeID[:]) < 0
	})
	view := [][]byte{accepted[0].raw, accepted[1].raw}
	inputRoot := verifierMerkleRoot(inputs, 0x10)
	viewRoot := verifierMerkleRoot(view, 0x11)
	rejectionRoot := verifierHashedRoot([][32]byte{
		verifierRejectionLeaf(1, 6, rejected.raw),
		verifierRejectionLeaf(3, 1, inputs[3]),
		verifierRejectionLeaf(4, 8, duplicate.raw),
		verifierRejectionLeaf(5, 8, duplicate.raw),
		verifierRejectionLeaf(6, 7, sourceCollision.raw),
		verifierRejectionLeaf(7, 4, future.raw),
	}, 0x12)

	unsigned := new(bytes.Buffer)
	unsigned.WriteString("AREP")
	unsigned.WriteByte(1)
	unsigned.Write(networkID[:])
	verifierU64(unsigned, 1)
	unsigned.Write(make([]byte, 32))
	verifierI64(unsigned, now-30)
	verifierI64(unsigned, now+1800)
	verifierU32(unsigned, uint32(len(inputs)))
	verifierText(unsigned, "h3-role-probe-v1")
	unsigned.Write(inputRoot[:])
	unsigned.Write(viewRoot[:])
	verifierU32(unsigned, 2)
	unsigned.Write(rejectionRoot[:])
	verifierU32(unsigned, 6)
	seed := sha256.Sum256([]byte("assignment-seed-1"))
	unsigned.Write(seed[:])
	verifierText(unsigned, "ardents-h3-role-domain-v1")
	verifierU32(unsigned, 2)
	verifierU32(unsigned, 8)
	verifierU16(unsigned, 2)
	verifierU16(unsigned, 1)
	verifierU32(unsigned, 5)
	unsigned.WriteByte(2)
	for _, summary := range verifierDomainSummaries(networkID, seed, accepted) {
		verifierText(unsigned, summary.id)
		verifierU16(unsigned, summary.count)
		verifierU32(unsigned, summary.capacity)
	}
	digest := sha256.Sum256(unsigned.Bytes())
	epoch := append([]byte(nil), unsigned.Bytes()...)
	epoch = append(epoch, 1)
	epoch = append(epoch, authorityID[:]...)
	epoch = append(epoch, ed25519.Sign(authorityPrivate, digest[:])...)
	generation := fmt.Sprintf("%x", digest)
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
	material := verifierMaterial(digest, 0, view[0], verifierProof(view, 0))
	return verifierFixture{root, now, networkID, authorityID, authorityPublic, authorityPrivate, generation, [][]byte{material}}
}

func writeVerifierSuccessor(t *testing.T, previous verifierFixture) verifierFixture {
	t.Helper()
	previousEpoch, err := os.ReadFile(filepath.Join(previous.root, "generations", previous.generation, "epoch.bin"))
	if err != nil {
		t.Fatal(err)
	}
	unsignedEnd := len(previousEpoch) - 1 - 32 - ed25519.SignatureSize
	unsigned := append([]byte(nil), previousEpoch[:unsignedEnd]...)
	previousDigest := sha256.Sum256(previousEpoch[:unsignedEnd])
	binary.BigEndian.PutUint64(unsigned[37:45], 2)
	copy(unsigned[45:77], previousDigest[:])
	digest := sha256.Sum256(unsigned)
	epoch := append(unsigned, 1)
	epoch = append(epoch, previous.authorityID[:]...)
	epoch = append(epoch, ed25519.Sign(previous.authorityPrivate, digest[:])...)
	generation := fmt.Sprintf("%x", digest)
	root := t.TempDir()
	directory := filepath.Join(root, "generations", generation, "inputs")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	previousInputs := filepath.Join(previous.root, "generations", previous.generation, "inputs")
	for index := range 8 {
		raw, readErr := os.ReadFile(filepath.Join(previousInputs, fmt.Sprintf("%04d.bin", index)))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("%04d.bin", index)), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "generations", generation, "epoch.bin"), epoch, 0o600); err != nil {
		t.Fatal(err)
	}
	material := append([]byte(nil), previous.materializations[0]...)
	copy(material[:32], digest[:])
	return verifierFixture{root, previous.now, previous.networkID, previous.authorityID, previous.authorityPublic,
		previous.authorityPrivate, generation, [][]byte{material}}
}

func verifierNodeRecord(network [32]byte, marker byte, family, endpoint string, capacity uint16, now int64) verifierRecord {
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	nodeID := sha256.Sum256([]byte{0x4e, marker})
	return verifierNodeRecordWithKey(network, nodeID, private, family, endpoint, capacity, now-60, now+3600)
}

func verifierNodeRecordAt(network [32]byte, marker byte, family, endpoint string, capacity uint16, notBefore, notAfter int64) verifierRecord {
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	nodeID := sha256.Sum256([]byte{0x4e, marker})
	return verifierNodeRecordWithKey(network, nodeID, private, family, endpoint, capacity, notBefore, notAfter)
}

func verifierNodeRecordWithKey(network, nodeID [32]byte, private ed25519.PrivateKey, family, endpoint string, capacity uint16, notBefore, notAfter int64) verifierRecord {
	public := private.Public().(ed25519.PublicKey)
	buf := new(bytes.Buffer)
	buf.WriteString("ARNR")
	buf.WriteByte(1)
	buf.Write(network[:])
	buf.Write(nodeID[:])
	verifierU64(buf, 1)
	verifierI64(buf, notBefore)
	verifierI64(buf, notAfter)
	verifierText(buf, family)
	buf.WriteByte(1)
	verifierText(buf, endpoint)
	verifierU16(buf, capacity)
	buf.Write(public)
	buf.Write(ed25519.Sign(private, buf.Bytes()))
	return verifierRecord{raw: buf.Bytes(), nodeID: nodeID, family: family, capacity: capacity}
}

type verifierDomainSummary struct {
	id       string
	count    uint16
	capacity uint32
}

func verifierDomainSummaries(network, seed [32]byte, records []verifierRecord) []verifierDomainSummary {
	domains := []verifierDomainSummary{{id: "alpha"}, {id: "beta"}}
	for _, record := range records {
		selected := 0
		first := verifierAssignmentDigest(network, seed, record.family, domains[0].id)
		second := verifierAssignmentDigest(network, seed, record.family, domains[1].id)
		if bytes.Compare(second[:], first[:]) < 0 {
			selected = 1
		}
		domains[selected].count++
		domains[selected].capacity += uint32(record.capacity)
	}
	return domains
}

func verifierAssignmentDigest(network, seed [32]byte, family, domain string) [32]byte {
	buf := new(bytes.Buffer)
	buf.WriteString("ardents-h3-role-domain-v1\x00")
	buf.Write(network[:])
	verifierU64(buf, 1)
	buf.Write(seed[:])
	buf.WriteString(family)
	buf.WriteString(domain)
	return sha256.Sum256(buf.Bytes())
}

func verifierMaterial(digest [32]byte, index uint32, record []byte, proof [][32]byte) []byte {
	buf := new(bytes.Buffer)
	buf.Write(digest[:])
	verifierU32(buf, index)
	verifierU32(buf, uint32(len(record)))
	buf.Write(record)
	verifierU16(buf, uint16(len(proof)))
	for _, sibling := range proof {
		buf.Write(sibling[:])
	}
	return buf.Bytes()
}

func verifierMerkleRoot(values [][]byte, empty byte) [32]byte {
	leaves := make([][32]byte, len(values))
	for index, value := range values {
		encoded := make([]byte, 5+len(value))
		binary.BigEndian.PutUint32(encoded[1:5], uint32(len(value)))
		copy(encoded[5:], value)
		leaves[index] = sha256.Sum256(encoded)
	}
	return verifierHashedRoot(leaves, empty)
}

func verifierHashedRoot(leaves [][32]byte, empty byte) [32]byte {
	if len(leaves) == 0 {
		return sha256.Sum256([]byte{empty})
	}
	if len(leaves) == 1 {
		return leaves[0]
	}
	split := verifierSplit(len(leaves))
	return verifierBranch(verifierHashedRoot(leaves[:split], empty), verifierHashedRoot(leaves[split:], empty))
}

func verifierRejectionLeaf(index uint32, code uint16, raw []byte) [32]byte {
	digest := sha256.Sum256(raw)
	encoded := make([]byte, 39)
	encoded[0] = 2
	binary.BigEndian.PutUint32(encoded[1:5], index)
	binary.BigEndian.PutUint16(encoded[5:7], code)
	copy(encoded[7:], digest[:])
	return sha256.Sum256(encoded)
}

func verifierProof(values [][]byte, index int) [][32]byte {
	if len(values) <= 1 {
		return nil
	}
	split := verifierSplit(len(values))
	if index < split {
		return append(verifierProof(values[:split], index), verifierMerkleRoot(values[split:], 0x11))
	}
	return append(verifierProof(values[split:], index-split), verifierMerkleRoot(values[:split], 0x11))
}

func verifierBranch(left, right [32]byte) [32]byte {
	encoded := append([]byte{1}, left[:]...)
	encoded = append(encoded, right[:]...)
	return sha256.Sum256(encoded)
}

func verifierSplit(length int) int {
	value := 1
	for value<<1 < length {
		value <<= 1
	}
	return value
}

func verifierText(buf *bytes.Buffer, value string) {
	buf.WriteByte(byte(len(value)))
	buf.WriteString(value)
}
func verifierU16(buf *bytes.Buffer, value uint16) {
	var out [2]byte
	binary.BigEndian.PutUint16(out[:], value)
	buf.Write(out[:])
}
func verifierU32(buf *bytes.Buffer, value uint32) {
	var out [4]byte
	binary.BigEndian.PutUint32(out[:], value)
	buf.Write(out[:])
}
func verifierU64(buf *bytes.Buffer, value uint64) {
	var out [8]byte
	binary.BigEndian.PutUint64(out[:], value)
	buf.Write(out[:])
}
func verifierI64(buf *bytes.Buffer, value int64) { verifierU64(buf, uint64(value)) }
