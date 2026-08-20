package nameresolution

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/nameauthority"
	"github.com/dianabuilds/ardents-network/internal/namelease"
)

func TestRecordSetRejectsAChainThatCannotFitTheFixedResponse(t *testing.T) {
	t.Parallel()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	record := namelease.Record{Name: "alice", Generation: 1, Revision: 1,
		Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: hex.EncodeToString(private.Public().(ed25519.PublicKey)), Target: strings.Repeat("t", fixedMessageSize),
		LeaseExpiresAt: 200, GraceExpiresAt: 220}
	signed, err := nameauthority.SignRecord([32]byte{1}, record, private)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newRecordSet([32]byte{1}, [][][]byte{{signed}}); err == nil {
		t.Fatal("oversized signed Record chain was accepted")
	}
}
