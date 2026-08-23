package epoch

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestLegacyTargetRecordFailsClosedInNewMaterialization(t *testing.T) {
	t.Parallel()
	network := [32]byte{6}
	seed := sha256.Sum256([]byte("legacy-record-authority"))
	private := ed25519.NewKeyFromSeed(seed[:])
	record := Record{Name: "alice", Generation: 1, Revision: 1, Lease: "active",
		Consistency: "current", Recovery: "stable",
		Authority: hex.EncodeToString(private.Public().(ed25519.PublicKey)), Target: [32]byte{1},
		LeaseExpiresAt: 1_000, GraceExpiresAt: 2_000}
	wire, err := EncodeRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint16(wire[:2], 3)
	wire = wire[:len(wire)-8]
	transcript := binary.BigEndian.AppendUint16(nil, uint16(len("ardents-name-record-v1")))
	transcript = append(transcript, "ardents-name-record-v1"...)
	transcript = append(transcript, network[:]...)
	transcript = binary.BigEndian.AppendUint64(transcript, uint64(len(wire)))
	transcript = append(transcript, wire...)
	signature := ed25519.Sign(private, transcript)
	signed := binary.BigEndian.AppendUint16(nil, 1)
	signed = binary.BigEndian.AppendUint64(signed, uint64(len(wire)))
	signed = append(signed, wire...)
	signed = append(signed, signature...)
	_, leaves, err := materializeRecords(network, [][]byte{signed})
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := decodeLeaf(leaves[0])
	if err != nil || leaf.schema != leafSchema || leaf.state != 0 || leaf.notAfter != 0 {
		t.Fatalf("legacy Target leaf=%+v err=%v", leaf, err)
	}
}
