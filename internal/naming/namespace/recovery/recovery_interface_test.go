package recovery

import (
	"bytes"
	"crypto/ed25519"
	"sort"
	"testing"
	"time"
)

func TestAuthorizeSealsOnlyCanonicalThresholdProof(t *testing.T) {
	t.Parallel()
	current := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{1}, ed25519.SeedSize))
	first := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{2}, ed25519.SeedSize))
	second := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{3}, ed25519.SeedSize))
	participants := [][32]byte{publicID(first), publicID(second)}
	sort.Slice(participants, func(left, right int) bool { return bytes.Compare(participants[left][:], participants[right][:]) < 0 })
	policy := RecoveryPolicy{Network: [32]byte{1}, Name: "alice", Generation: 1, Revision: 1,
		CurrentAuthority: publicID(current), Threshold: 2, Participants: participants, Delay: 72 * time.Hour}
	proof := RecoveryProof{Operation: "initiate", PolicyDigest: policy.Digest(), OperationID: [32]byte{4},
		Successor: [32]byte{5}, StartedAt: time.Unix(100, 0).UnixMilli(), CompletesAt: time.Unix(100, 0).Add(72 * time.Hour).UnixMilli()}
	transcript := policy.Transcript(proof)
	for _, private := range []ed25519.PrivateKey{first, second} {
		proof.Signatures = append(proof.Signatures, Signature{Signer: publicID(private), Bytes: ed25519.Sign(private, transcript)})
	}
	sort.Slice(proof.Signatures, func(left, right int) bool {
		return bytes.Compare(proof.Signatures[left].Signer[:], proof.Signatures[right].Signer[:]) < 0
	})
	authorization, err := policy.Authorize(proof)
	if err != nil || !authorization.Verified() || authorization.ValidSigners != 2 {
		t.Fatalf("canonical threshold proof was not authorized: authorization=%+v err=%v", authorization, err)
	}
	proof.Signatures[1] = proof.Signatures[0]
	if _, err := policy.Authorize(proof); err == nil {
		t.Fatal("duplicate signer was authorized")
	}
}

func publicID(private ed25519.PrivateKey) [32]byte {
	var id [32]byte
	copy(id[:], private.Public().(ed25519.PublicKey))
	return id
}
