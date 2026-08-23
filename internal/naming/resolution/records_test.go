package resolution_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

func TestRecordTooLargeForTheFixedResponseIsRejectedBeforeMaterialization(t *testing.T) {
	t.Parallel()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	record := namespace.Record{Name: "alice", Generation: 1, Revision: 1,
		Lease: "active", Consistency: "conflict", Recovery: "stable",
		Authority: hex.EncodeToString(private.Public().(ed25519.PublicKey)), Target: [32]byte{1},
		ConflictIdentifier: strings.Repeat("t", 4096),
		LeaseExpiresAt:     200, GraceExpiresAt: 220}
	signed, err := namespace.SignRecord([32]byte{1}, record, private)
	if err == nil || signed != nil {
		t.Fatalf("oversized Record signed=%d err=%v", len(signed), err)
	}
}
