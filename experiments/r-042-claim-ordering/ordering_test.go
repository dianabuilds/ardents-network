//go:build ignore

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"testing"
)

func TestVerifyAcceptsEarliestEligibleCommitment(t *testing.T) {
	t.Parallel()
	firstPublic, firstPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	secondPublic, secondPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	first := signedClaim(t, 4, "alice", [32]byte{1}, firstPublic, firstPrivate)
	second := signedClaim(t, 9, "alice", [32]byte{2}, secondPublic, secondPrivate)

	policy, proof := authenticatedProof(t, []Claim{second, first})
	result, err := Verify(policy, proof)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Outcome != Accepted || result.WinnerOrdinal != 4 || len(result.LoserOrdinals) != 1 ||
		result.LoserOrdinals[0] != 9 {
		t.Fatalf("result = %+v", result)
	}
}

func TestVerifyRejectsUnauthenticatedCompleteness(t *testing.T) {
	t.Parallel()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	claim := signedClaim(t, 1, "alice", [32]byte{3}, public, private)
	result, err := Verify(Policy{Network: [32]byte{7}, Rule: "ardents-name-claim-order-v1",
		MinimumEpoch: 11, MaxClaims: 32, Threshold: 2}, ClaimSetProof{Network: [32]byte{7}, Epoch: 11,
		Rule: "ardents-name-claim-order-v1", Complete: true, Claims: []Claim{claim}})
	if err == nil || result.Outcome != Unavailable {
		t.Fatalf("unauthenticated result = %+v, err = %v", result, err)
	}
}

func TestVerifyRejectsEpochRollback(t *testing.T) {
	t.Parallel()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	policy, proof := authenticatedProof(t, []Claim{
		signedClaim(t, 1, "alice", [32]byte{4}, public, private),
	})
	policy.MinimumEpoch = proof.Epoch + 1
	result, err := Verify(policy, proof)
	if err == nil || result.Outcome != Unavailable {
		t.Fatalf("rollback result = %+v, err = %v", result, err)
	}
}

func TestVerifyClassifiesTwoAuthenticatedRootsAsFork(t *testing.T) {
	t.Parallel()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	policy, proof, signers := authenticatedProofWithSigners(t, []Claim{
		signedClaim(t, 1, "alice", [32]byte{5}, public, private),
	})
	alternate := SetStatement{Root: proof.SetRoot, Length: uint32(len(proof.Claims))}
	alternate.Root[0] ^= 0xff
	alternate.Signatures = signSetForTest(proof, alternate, signers)
	proof.AlternateSets = []SetStatement{alternate}

	result, err := Verify(policy, proof)
	if err != nil || result.Outcome != Fork {
		t.Fatalf("fork result = %+v, err = %v", result, err)
	}
}

func TestVerifyClassifiesAuthenticatedRuleChangeAsFork(t *testing.T) {
	t.Parallel()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	policy, proof, signers := authenticatedProofWithSigners(t, []Claim{
		signedClaim(t, 1, "alice", [32]byte{6}, public, private),
	})
	proof.Rule = "ardents-name-claim-order-v2"
	proof.SetSignatures = signSetForTest(proof, SetStatement{Root: proof.SetRoot,
		Length: uint32(len(proof.Claims))}, signers)

	result, err := Verify(policy, proof)
	if err != nil || result.Outcome != Fork {
		t.Fatalf("rule-fork result = %+v, err = %v", result, err)
	}
}

func TestVerifyHostileClaimMatrixFailsClosed(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	base := signedClaim(t, 3, "alice", [32]byte{7}, public, private)

	t.Run("copied reveal at another ordinal", func(t *testing.T) {
		copied := base
		copied.Ordinal++
		policy, proof := authenticatedProof(t, []Claim{base, copied})
		assertOutcome(t, policy, proof, Conflict)
	})
	t.Run("withheld completeness", func(t *testing.T) {
		policy, proof := authenticatedProof(t, []Claim{base})
		proof.Complete = false
		assertOutcome(t, policy, proof, Unavailable)
	})
	t.Run("claim flood beyond cap", func(t *testing.T) {
		claims := make([]Claim, 33)
		for i := range claims {
			claims[i] = signedClaim(t, uint32(i+1), "alice", [32]byte{byte(i + 1)}, public, private)
		}
		policy, proof := authenticatedProof(t, claims)
		assertOutcome(t, policy, proof, Unavailable)
	})
	t.Run("mixed names", func(t *testing.T) {
		other := signedClaim(t, 4, "bob", [32]byte{8}, public, private)
		policy, proof := authenticatedProof(t, []Claim{base, other})
		assertOutcome(t, policy, proof, Conflict)
	})
	t.Run("duplicate threshold signer", func(t *testing.T) {
		policy, proof := authenticatedProof(t, []Claim{base})
		proof.SetSignatures[1] = proof.SetSignatures[0]
		assertOutcome(t, policy, proof, Unavailable)
	})
}

func TestVerifyIsIndependentOfRevealObservationOrder(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	claims := []Claim{
		signedClaim(t, 21, "alice", [32]byte{9}, public, private),
		signedClaim(t, 2, "alice", [32]byte{10}, public, private),
		signedClaim(t, 13, "alice", [32]byte{11}, public, private),
	}
	for _, order := range [][]int{{0, 1, 2}, {2, 0, 1}, {1, 2, 0}, {2, 1, 0}} {
		permuted := []Claim{claims[order[0]], claims[order[1]], claims[order[2]]}
		policy, proof := authenticatedProof(t, permuted)
		result, err := Verify(policy, proof)
		if err != nil || result.Outcome != Accepted || result.WinnerOrdinal != 2 {
			t.Fatalf("order %v result = %+v, err = %v", order, result, err)
		}
	}
}

func assertOutcome(t *testing.T, policy Policy, proof ClaimSetProof, want Outcome) {
	t.Helper()
	result, err := Verify(policy, proof)
	if result.Outcome != want {
		t.Fatalf("result = %+v, err = %v, want %s", result, err, want)
	}
	if want == Accepted || want == Fork {
		if err != nil {
			t.Fatalf("classified result returned structural error: %v", err)
		}
	} else if err == nil {
		t.Fatal("fail-closed result returned no error")
	}
}

func signedClaim(t *testing.T, ordinal uint32, name string, secret [32]byte,
	public ed25519.PublicKey, private ed25519.PrivateKey) Claim {
	t.Helper()
	claim := Claim{Ordinal: ordinal, Name: name, Secret: secret}
	copy(claim.Authority[:], public)
	claim.Commitment = CommitmentFor([32]byte{7}, 11, claim)
	claim.Signature = ed25519.Sign(private, RevealTranscript([32]byte{7}, 11, claim))
	return claim
}

func authenticatedProof(t *testing.T, claims []Claim) (Policy, ClaimSetProof) {
	policy, proof, _ := authenticatedProofWithSigners(t, claims)
	return policy, proof
}

func authenticatedProofWithSigners(t *testing.T, claims []Claim) (Policy, ClaimSetProof, []ed25519.PrivateKey) {
	t.Helper()
	proof := ClaimSetProof{Network: [32]byte{7}, Epoch: 11,
		Rule: "ardents-name-claim-order-v1", Complete: true, Claims: claims}
	policy := Policy{Network: proof.Network, Rule: proof.Rule, MinimumEpoch: proof.Epoch, MaxClaims: 32,
		Authorities: make(map[[32]byte]ed25519.PublicKey), Threshold: 2}
	proof.SetRoot = claimSetRootForTest(claims)
	var signers []ed25519.PrivateKey
	for marker := byte(1); marker <= 2; marker++ {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		id := sha256.Sum256(public)
		policy.Authorities[id] = public
		signers = append(signers, private)
		proof.SetSignatures = append(proof.SetSignatures, SetSignature{AuthorityID: id,
			Signature: ed25519.Sign(private, claimSetTranscriptForTest(proof))})
	}
	sort.Slice(proof.SetSignatures, func(i, j int) bool {
		return bytes.Compare(proof.SetSignatures[i].AuthorityID[:], proof.SetSignatures[j].AuthorityID[:]) < 0
	})
	return policy, proof, signers
}

func signSetForTest(proof ClaimSetProof, statement SetStatement, signers []ed25519.PrivateKey) []SetSignature {
	copyProof := proof
	copyProof.SetRoot = statement.Root
	copyProof.Claims = make([]Claim, statement.Length)
	var signatures []SetSignature
	for _, private := range signers {
		public := private.Public().(ed25519.PublicKey)
		id := sha256.Sum256(public)
		signatures = append(signatures, SetSignature{AuthorityID: id,
			Signature: ed25519.Sign(private, claimSetTranscriptForTest(copyProof))})
	}
	sort.Slice(signatures, func(i, j int) bool {
		return bytes.Compare(signatures[i].AuthorityID[:], signatures[j].AuthorityID[:]) < 0
	})
	return signatures
}

func claimSetRootForTest(claims []Claim) [32]byte {
	claims = append([]Claim(nil), claims...)
	sort.Slice(claims, func(i, j int) bool { return claims[i].Ordinal < claims[j].Ordinal })
	leaves := make([][32]byte, len(claims))
	for i, claim := range claims {
		leaf := []byte{0}
		leaf = binary.BigEndian.AppendUint32(leaf, claim.Ordinal)
		leaf = append(leaf, claim.Commitment[:]...)
		leaves[i] = sha256.Sum256(leaf)
	}
	return merkleRootForTest(leaves)
}

func merkleRootForTest(leaves [][32]byte) [32]byte {
	if len(leaves) == 1 {
		return leaves[0]
	}
	split := 1
	for split<<1 < len(leaves) {
		split <<= 1
	}
	left, right := merkleRootForTest(leaves[:split]), merkleRootForTest(leaves[split:])
	node := append([]byte{1}, left[:]...)
	node = append(node, right[:]...)
	return sha256.Sum256(node)
}

func claimSetTranscriptForTest(proof ClaimSetProof) []byte {
	out := binary.BigEndian.AppendUint32(nil, uint32(len("ardents-name-claim-set-v1")))
	out = append(out, "ardents-name-claim-set-v1"...)
	out = append(out, proof.Network[:]...)
	out = binary.BigEndian.AppendUint64(out, proof.Epoch)
	out = binary.BigEndian.AppendUint32(out, uint32(len(proof.Rule)))
	out = append(out, proof.Rule...)
	out = append(out, proof.SetRoot[:]...)
	return binary.BigEndian.AppendUint32(out, uint32(len(proof.Claims)))
}
