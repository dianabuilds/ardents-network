package namespace

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
	record := Record{Name: "alice", Generation: 1, Revision: 1, Lease: leaseActive,
		Consistency: consistencyCurrent, Recovery: recoveryStable,
		Authority: hex.EncodeToString(private.Public().(ed25519.PublicKey)), Target: [32]byte{1},
		LeaseExpiresAt: 1_000, GraceExpiresAt: 2_000}
	wire, err := encodeRecord(record, legacyRecordSchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	signature := ed25519.Sign(private, recordTranscript(network, wire))
	signed := binary.BigEndian.AppendUint16(nil, signedRecordSchema)
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
