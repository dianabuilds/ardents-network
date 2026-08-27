package service_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/tests/epochfixture/assignment"
	"github.com/dianabuilds/ardents-network/tests/epochfixture/merkle"
)

type referenceC2DomainSummary struct {
	id       string
	count    uint16
	capacity uint32
}

func referenceC2BuildStateEpoch(network [32]byte, epoch uint64, previous [32]byte, now, deadline time.Time, inputs [][]byte,
	records []referenceC2StateRecord, seed [32]byte, domains []string, authority ed25519.PrivateKey) ([]byte, [32]byte, [][]byte, error) {
	if epoch == 0 || len(records) == 0 || len(records) > 64 || len(inputs) != len(records) || len(domains) == 0 || len(authority) != ed25519.PrivateKeySize {
		return nil, [32]byte{}, nil, errors.New("reference C2 State Epoch input is invalid")
	}
	view := make([][]byte, len(records))
	for index := range records {
		view[index] = records[index].raw
	}
	summaries, familyCount, capacity, maximumFamilyCount, maximumFamilyCapacity, err := referenceC2StateSummaries(network, epoch, seed, records, domains)
	if err != nil {
		return nil, [32]byte{}, nil, err
	}
	unsigned := referenceC2EncodeStateEpoch(network, epoch, previous, now, deadline, inputs, view, seed, summaries,
		familyCount, capacity, maximumFamilyCount, maximumFamilyCapacity)
	digest := sha256.Sum256(unsigned.Bytes())
	raw := referenceC2SignStateEpoch(unsigned.Bytes(), digest, authority)
	materials := make([][]byte, len(view))
	for index, record := range view {
		materials[index] = referenceC2StateMaterial(digest, uint32(index), record, merkle.Proof(view, index, 0x11))
	}
	return raw, digest, materials, nil
}

func referenceC2StateSummaries(network [32]byte, epoch uint64, seed [32]byte, records []referenceC2StateRecord,
	domains []string) ([]referenceC2DomainSummary, uint16, uint32, uint16, uint32, error) {
	summaries := make([]referenceC2DomainSummary, len(domains))
	for index, domain := range domains {
		summaries[index].id = domain
	}
	families := make(map[string][2]uint32, len(records))
	var total uint32
	for _, record := range records {
		selected, err := assignment.Select(network, epoch, seed, record.family, domains)
		if err != nil {
			return nil, 0, 0, 0, 0, err
		}
		for index := range summaries {
			if summaries[index].id == selected {
				summaries[index].count++
				summaries[index].capacity += uint32(record.capacity)
			}
		}
		family := families[record.family]
		family[0]++
		family[1] += uint32(record.capacity)
		families[record.family] = family
		total += uint32(record.capacity)
	}
	var maximumCount uint32
	var maximumCapacity uint32
	for _, family := range families {
		maximumCount = max(maximumCount, family[0])
		maximumCapacity = max(maximumCapacity, family[1])
	}
	return summaries, uint16(len(families)), total, uint16(maximumCount), maximumCapacity, nil
}

func referenceC2EncodeStateEpoch(network [32]byte, epoch uint64, previous [32]byte, now, deadline time.Time, inputs, view [][]byte, seed [32]byte,
	summaries []referenceC2DomainSummary, familyCount uint16, capacity uint32, maximumFamilyCount uint16, maximumFamilyCapacity uint32) *bytes.Buffer {
	buffer := new(bytes.Buffer)
	buffer.WriteString("AREP")
	buffer.WriteByte(1)
	buffer.Write(network[:])
	referenceC2U64(buffer, epoch)
	buffer.Write(previous[:])
	referenceC2I64(buffer, now.Add(-time.Minute).Unix())
	referenceC2I64(buffer, deadline.Unix())
	referenceC2U32(buffer, uint32(len(inputs)))
	referenceC2Text(buffer, "ardents-interactive-route-v1")
	inputRoot := merkle.Root(inputs, 0x10)
	viewRoot := merkle.Root(view, 0x11)
	emptyRejected := merkle.HashedRoot(nil, 0x12)
	buffer.Write(inputRoot[:])
	buffer.Write(viewRoot[:])
	referenceC2U32(buffer, uint32(len(view)))
	buffer.Write(emptyRejected[:])
	referenceC2U32(buffer, 0)
	buffer.Write(seed[:])
	referenceC2Text(buffer, "ardents-h3-role-domain-v1")
	referenceC2U32(buffer, uint32(len(view)))
	referenceC2U32(buffer, capacity)
	referenceC2U16(buffer, familyCount)
	referenceC2U16(buffer, maximumFamilyCount)
	referenceC2U32(buffer, maximumFamilyCapacity)
	buffer.WriteByte(byte(len(summaries)))
	for _, summary := range summaries {
		referenceC2Text(buffer, summary.id)
		referenceC2U16(buffer, summary.count)
		referenceC2U32(buffer, summary.capacity)
	}
	return buffer
}

func referenceC2SignStateEpoch(unsigned []byte, digest [32]byte, authority ed25519.PrivateKey) []byte {
	public := authority.Public().(ed25519.PublicKey)
	id := sha256.Sum256(public)
	raw := append([]byte(nil), unsigned...)
	raw = append(raw, 1)
	raw = append(raw, id[:]...)
	raw = append(raw, ed25519.Sign(authority, digest[:])...)
	return raw
}

func referenceC2StateMaterial(digest [32]byte, index uint32, record []byte, siblings [][32]byte) []byte {
	buffer := new(bytes.Buffer)
	buffer.Write(digest[:])
	referenceC2U32(buffer, index)
	referenceC2U32(buffer, uint32(len(record)))
	buffer.Write(record)
	referenceC2U16(buffer, uint16(len(siblings)))
	for _, sibling := range siblings {
		buffer.Write(sibling[:])
	}
	return buffer.Bytes()
}
