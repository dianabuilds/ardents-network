package epochfixture

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"sort"

	"github.com/dianabuilds/ardents-network/internal/network/epoch/merkle"
)

func encodeEpochCommitment(spec EpochSpec, view [][]byte, rejected [][32]byte, summaries []domainSummary,
	families uint16, capacity uint32, maxFamilyCount uint16, maxFamilyCapacity uint32) *bytes.Buffer {
	buffer := new(bytes.Buffer)
	buffer.WriteString("AREP")
	buffer.WriteByte(1)
	buffer.Write(spec.NetworkID[:])
	u64(buffer, spec.Number)
	buffer.Write(spec.Previous[:])
	i64(buffer, spec.ValidFrom.Unix())
	i64(buffer, spec.ValidUntil.Unix())
	u32(buffer, uint32(len(spec.Inputs)))
	text(buffer, "h3-role-probe-v1")
	inputRoot, viewRoot := merkle.Root(spec.Inputs, 0x10), merkle.Root(view, 0x11)
	buffer.Write(inputRoot[:])
	buffer.Write(viewRoot[:])
	u32(buffer, uint32(len(view)))
	rejectedRoot := merkle.HashedRoot(rejected, 0x12)
	buffer.Write(rejectedRoot[:])
	u32(buffer, uint32(len(rejected)))
	buffer.Write(spec.AssignmentSeed[:])
	text(buffer, "ardents-h3-role-domain-v1")
	u32(buffer, uint32(len(view)))
	u32(buffer, capacity)
	u16(buffer, families)
	u16(buffer, maxFamilyCount)
	u32(buffer, maxFamilyCapacity)
	buffer.WriteByte(byte(len(summaries)))
	for _, value := range summaries {
		text(buffer, value.id)
		u16(buffer, value.count)
		u32(buffer, value.capacity)
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

func material(digest [32]byte, index uint32, record []byte, siblings [][32]byte) []byte {
	buffer := new(bytes.Buffer)
	buffer.Write(digest[:])
	u32(buffer, index)
	u32(buffer, uint32(len(record)))
	buffer.Write(record)
	u16(buffer, uint16(len(siblings)))
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
