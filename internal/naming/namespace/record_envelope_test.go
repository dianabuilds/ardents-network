package namespace

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestRecordSigningRejectsPayloadThatCannotFitCurrentProofEnvelope(t *testing.T) {
	t.Parallel()
	seed := sha256.Sum256([]byte("record-envelope"))
	key := ed25519.NewKeyFromSeed(seed[:])
	record := Record{Name: "alice", Generation: 1, Revision: 1, Lease: leaseActive,
		Consistency: consistencyConflict, Recovery: recoveryStable,
		Authority:      hex.EncodeToString(key.Public().(ed25519.PublicKey)),
		LeaseExpiresAt: 1_000, GraceExpiresAt: 2_000, Continuity: 1, ConflictIdentifier: "x"}
	wire, err := EncodeRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	record.ConflictIdentifier = strings.Repeat("x", maximumRecordPayloadBytes-len(wire)+1)
	wire, err = EncodeRecord(record)
	if err != nil || len(wire) != maximumRecordPayloadBytes {
		t.Fatalf("payload bytes=%d err=%v", len(wire), err)
	}
	signed, err := SignRecord([32]byte{7}, record, key)
	if err != nil || len(signed) != maximumSignedRecordBytes {
		t.Fatalf("signed bytes=%d err=%v", len(signed), err)
	}
	record.ConflictIdentifier += "x"
	if _, err := SignRecord([32]byte{7}, record, key); err == nil {
		t.Fatal("Record beyond the fixed proof envelope was signed")
	}
}
