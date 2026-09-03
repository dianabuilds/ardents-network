//go:build ignore

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"sort"

	"github.com/dianabuilds/ardents-network/tests/epochfixture/assignment"
	"github.com/dianabuilds/ardents-network/tests/epochfixture/merkle"
)

// Epoch is one canonical signed Network State Epoch, byte-compatible with the
// maintained Epoch type the source-server accept-offline path parses.
type Epoch struct {
	Number    uint64
	Seed      [32]byte
	Raw       []byte
	Digest    [32]byte
	Inputs    [][]byte
	Materials [][]byte
}

type domainSummary struct {
	id       string
	count    uint16
	capacity uint32
}

// BuildEpoch returns one canonical signed Epoch. The encoding is identical to
// the maintained BuildEpoch under tests/e2e/network-source, kept here so a
// non-test Go binary can produce a closed-alpha fixture that the source
// server's accept-offline path will accept unchanged.
func BuildEpoch(network [32]byte, number uint64, previous [32]byte, validFrom, validUntil int64,
	inputs [][]byte, accepted []Record, rejections map[uint32]uint16, assignmentSeed [32]byte,
	domains []string, authorities []ed25519.PrivateKey) (Epoch, error) {
	if number == 0 || validFrom == 0 || validUntil <= validFrom || len(inputs) > 64 || len(accepted) > 64 ||
		len(rejections) > 64 || len(domains) == 0 || len(domains) > 16 ||
		len(authorities) == 0 || len(authorities) > 16 {
		return Epoch{}, errors.New("pilot: epoch specification is invalid")
	}
	for _, domain := range domains {
		if domain == "" || len(domain) > 32 {
			return Epoch{}, errors.New("pilot: epoch domain is invalid")
		}
	}
	for _, private := range authorities {
		if len(private) != ed25519.PrivateKeySize {
			return Epoch{}, errors.New("pilot: epoch authority is invalid")
		}
	}
	sorted := append([]Record(nil), accepted...)
	sort.Slice(sorted, func(i, j int) bool { return bytes.Compare(sorted[i].NodeID[:], sorted[j].NodeID[:]) < 0 })
	view := make([][]byte, len(sorted))
	for index := range sorted {
		view[index] = sorted[index].Raw
	}
	indices := make([]int, 0, len(rejections))
	for index := range rejections {
		indices = append(indices, int(index))
	}
	sort.Ints(indices)
	rejectionLeaves := make([][32]byte, 0, len(indices))
	for _, index := range indices {
		if index < 0 || index >= len(inputs) {
			return Epoch{}, errors.New("pilot: rejection index is outside inputs")
		}
		rejectionLeaves = append(rejectionLeaves, merkle.RejectionLeaf(uint32(index), rejections[uint32(index)], inputs[index]))
	}
	summaries, families, capacity, maxFamilyCount, maxFamilyCapacity, err := summarize(network, number, assignmentSeed, domains, sorted)
	if err != nil {
		return Epoch{}, err
	}
	commitment := encodeEpochCommitment(network, number, previous, validFrom, validUntil, inputs, view, rejectionLeaves,
		summaries, families, capacity, maxFamilyCount, maxFamilyCapacity, assignmentSeed)
	digest := sha256.Sum256(commitment.Bytes())
	raw, err := signEpoch(commitment.Bytes(), digest, authorities)
	if err != nil {
		return Epoch{}, err
	}
	materials := make([][]byte, len(view))
	for index, record := range view {
		materials[index] = buildMaterial(digest, uint32(index), record, merkle.Proof(view, index, 0x11))
	}
	return Epoch{Number: number, Seed: assignmentSeed, Raw: raw, Digest: digest,
		Inputs: cloneBytes(inputs), Materials: materials}, nil
}

func summarize(network [32]byte, number uint64, seed [32]byte, domains []string, records []Record) (
	[]domainSummary, uint16, uint32, uint16, uint32, error) {
	values := make([]domainSummary, len(domains))
	for index, domain := range domains {
		values[index].id = domain
	}
	families := make(map[string][2]uint32)
	var capacity uint32
	for _, record := range records {
		selected, err := assignment.Select(network, number, seed, record.Family, domains)
		if err != nil {
			return nil, 0, 0, 0, 0, err
		}
		for index := range values {
			if values[index].id == selected {
				values[index].count++
				values[index].capacity += uint32(record.Capacity)
			}
		}
		family := families[record.Family]
		family[0]++
		family[1] += uint32(record.Capacity)
		families[record.Family] = family
		capacity += uint32(record.Capacity)
	}
	var maxCount, maxCapacity uint32
	for _, family := range families {
		if family[0] > maxCount {
			maxCount = family[0]
		}
		if family[1] > maxCapacity {
			maxCapacity = family[1]
		}
	}
	return values, uint16(len(families)), capacity, uint16(maxCount), maxCapacity, nil
}

func encodeEpochCommitment(network [32]byte, number uint64, previous [32]byte, validFrom, validUntil int64,
	inputs [][]byte, view [][]byte, rejected [][32]byte, summaries []domainSummary, families uint16,
	capacity uint32, maxFamilyCount uint16, maxFamilyCapacity uint32, assignmentSeed [32]byte) *bytes.Buffer {
	buffer := new(bytes.Buffer)
	buffer.WriteString("AREP")
	buffer.WriteByte(1)
	buffer.Write(network[:])
	writeU64(buffer, number)
	buffer.Write(previous[:])
	writeI64(buffer, validFrom)
	writeI64(buffer, validUntil)
	writeU32(buffer, uint32(len(inputs)))
	writeText(buffer, "h3-role-probe-v1")
	inputRoot := merkle.Root(inputs, 0x10)
	viewRoot := merkle.Root(view, 0x11)
	buffer.Write(inputRoot[:])
	buffer.Write(viewRoot[:])
	writeU32(buffer, uint32(len(view)))
	rejectedRoot := merkle.HashedRoot(rejected, 0x12)
	buffer.Write(rejectedRoot[:])
	writeU32(buffer, uint32(len(rejected)))
	buffer.Write(assignmentSeed[:])
	writeText(buffer, "ardents-h3-role-domain-v1")
	writeU32(buffer, uint32(len(view)))
	writeU32(buffer, capacity)
	writeU16(buffer, families)
	writeU16(buffer, maxFamilyCount)
	writeU32(buffer, maxFamilyCapacity)
	buffer.WriteByte(byte(len(summaries)))
	for _, value := range summaries {
		writeText(buffer, value.id)
		writeU16(buffer, value.count)
		writeU32(buffer, value.capacity)
	}
	return buffer
}

func signEpoch(unsigned []byte, digest [32]byte, authorities []ed25519.PrivateKey) ([]byte, error) {
	type signed struct {
		id        [32]byte
		signature []byte
	}
	values := make([]signed, len(authorities))
	for index, private := range authorities {
		public := private.Public().(ed25519.PublicKey)
		values[index] = signed{sha256.Sum256(public), ed25519.Sign(private, digest[:])}
	}
	sort.Slice(values, func(i, j int) bool { return bytes.Compare(values[i].id[:], values[j].id[:]) < 0 })
	raw := append([]byte(nil), unsigned...)
	raw = append(raw, byte(len(values)))
	for _, value := range values {
		raw = append(raw, value.id[:]...)
		raw = append(raw, value.signature...)
	}
	return raw, nil
}

func buildMaterial(digest [32]byte, index uint32, record []byte, siblings [][32]byte) []byte {
	buffer := new(bytes.Buffer)
	buffer.Write(digest[:])
	writeU32(buffer, index)
	writeU32(buffer, uint32(len(record)))
	buffer.Write(record)
	writeU16(buffer, uint16(len(siblings)))
	for _, sibling := range siblings {
		buffer.Write(sibling[:])
	}
	return buffer.Bytes()
}

func cloneBytes(values [][]byte) [][]byte {
	cloned := make([][]byte, len(values))
	for index := range values {
		cloned[index] = append([]byte(nil), values[index]...)
	}
	return cloned
}
