package closedprofile

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"testing"
	"time"
)

// The shared canonical cursor (R5, #804) must keep the signed-profile grammar's
// exact wrapped refusals: cursor truncation never surfaces its own prose here,
// each field read keeps the profile's field-specific refusal.
func TestClosedProfileTruncationKeepsWrappedRefusals(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	network := sha256.Sum256([]byte("network"))
	generation := sha256.Sum256([]byte("generation"))
	epochDigest := sha256.Sum256([]byte("epoch"))
	first := sha256.Sum256([]byte("first"))
	second := sha256.Sum256([]byte("second"))
	nodes := []Node{
		{NodeID: first, RecordDigest: sha256.Sum256([]byte("record-first")), RoleDomain: 1, Subrole: 1, DutyGeneration: 1},
		{NodeID: second, RecordDigest: sha256.Sum256([]byte("record-second")), RoleDomain: 2, Subrole: 6, DutyGeneration: 2},
	}
	if bytes.Compare(nodes[0].NodeID[:], nodes[1].NodeID[:]) > 0 {
		nodes[0], nodes[1] = nodes[1], nodes[0]
	}
	raw := testClosedProfile(t, authority, network, generation, epochDigest, now, nodes)
	body := raw[:len(raw)-ed25519.SignatureSize]
	for _, test := range []struct {
		name      string
		keep      int
		wantError string
	}{
		{name: "identity", keep: len(profileMagic) + 2 + 16, wantError: "closed profile identity is invalid"},
		{name: "node", keep: len(profileMagic) + 2 + 32 + 32 + 8 + 32 + 8 + 8 + 32 + 32 + 2 + 16, wantError: "closed profile node is invalid"},
	} {
		t.Run(test.name, func(t *testing.T) {
			truncated := body[:test.keep]
			signed := append(append([]byte(nil), truncated...), ed25519.Sign(authority, signingTranscript(truncated))...)
			if _, err := verifyFixture(signed, generation, network, epochDigest, 9, authority.Public().(ed25519.PublicKey), now); err == nil || err.Error() != test.wantError {
				t.Fatalf("truncated %s refusal = %v, want %q", test.name, err, test.wantError)
			}
		})
	}
}
