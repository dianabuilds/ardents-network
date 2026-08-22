package nameresolution_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/nameauthority"
	"github.com/dianabuilds/ardents-network/internal/namestore"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

func TestMaterializedRecordRejectsAProofThatCannotFitTheFixedResponse(t *testing.T) {
	t.Parallel()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	record := namespace.Record{Name: "alice", Generation: 1, Revision: 1,
		Lease: "active", Consistency: "conflict", Recovery: "stable",
		Authority: hex.EncodeToString(private.Public().(ed25519.PublicKey)), Target: [32]byte{1},
		ConflictIdentifier: strings.Repeat("t", 4096),
		LeaseExpiresAt:     200, GraceExpiresAt: 220}
	signed, err := nameauthority.SignRecord([32]byte{1}, record, private)
	if err != nil {
		t.Fatal(err)
	}
	materialization := testNamespaceFixture([32]byte{1}, "oversized-proof")
	store, err := namestore.Open(t.TempDir(), materialization.policy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	materialization.commit(t, store, 1, [][]byte{signed})
	if _, err := store.Lookup("alice", 1); err == nil {
		t.Fatal("oversized signed Record proof was accepted")
	}
}
