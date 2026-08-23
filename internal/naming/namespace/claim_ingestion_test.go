package namespace_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

func TestClaimCommitmentAdmissionIsSpentAndRevealDerivesItsDigest(t *testing.T) {
	t.Parallel()
	network := [32]byte{11}
	gate, err := namespace.NewAdmission([32]byte{1}, network, 7, [32]byte{2})
	if err != nil {
		t.Fatal(err)
	}
	key := deterministicAuthority("claim-ingestion")
	claim := namespace.Claim{Name: "alice", Secret: [32]byte{3}}
	copy(claim.Authority[:], key.Public().(ed25519.PublicKey))
	commitment := namespace.CommitmentFor(network, 7, claim)
	challenge, err := gate.Issue(100, "root-claim", commitment, [32]byte{4}, 1_000, [16]byte{5})
	if err != nil {
		t.Fatal(err)
	}
	proof, _ := challenge.Solve()
	accepted, err := namespace.AdmitClaimCommitment(gate, 100, commitment, proof)
	if err != nil {
		t.Fatal(err)
	}
	copy(claim.Signature[:], ed25519.Sign(key, namespace.RevealTranscript(network, 7, namespace.Claim{Commitment: commitment})))
	reveal, err := accepted.Reveal(claim.Name, claim.Secret, claim.Authority, claim.Signature)
	if err != nil || reveal.AdmissionDigest == [32]byte{} {
		t.Fatalf("reveal=%+v err=%v", reveal, err)
	}
	if _, err := namespace.AdmitClaimCommitment(gate, 100, commitment, proof); err == nil {
		t.Fatal("replayed proof admitted another commitment")
	}
	if _, err := accepted.Reveal("bob", claim.Secret, claim.Authority, claim.Signature); err == nil {
		t.Fatal("signature opened a different Name")
	}
	input, err := accepted.EpochInput()
	if err != nil || input.Commitment() != commitment || len(input.Canonical()) != 64 ||
		input.InputLeaf(0) != orderedInputLeaf(reveal) {
		t.Fatalf("Epoch input=%x commitment=%x err=%v", input.Canonical(), input.Commitment(), err)
	}
	close := namespace.ClaimProof{Network: network, Epoch: 7, Rule: "ardents-name-claim-order-v1",
		CutoffOffset: 1_000, InputRoot: input.InputLeaf(0), InputLength: 1,
		MaterializationRoot: orderedMaterializationLeaf(reveal), MaterializationLength: 1,
		RejectionRoot: sha256.Sum256([]byte{2}), Claims: []namespace.Claim{reveal}}
	if _, err := (namespace.EpochClaimInput{}).VerifyClose(namespace.ClaimOrder{}, 0, close); err == nil {
		t.Fatal("caller-built zero Epoch input verified a close")
	}
	if _, err := input.VerifyClose(signedClaimClose(&close), 0, close); err != nil {
		t.Fatalf("admitted input did not yield verified winner: %v", err)
	}
	substituted := reveal
	substituted.AdmissionDigest = [32]byte{9}
	forged := namespace.ClaimProof{Network: network, Epoch: 7, Rule: "ardents-name-claim-order-v1",
		CutoffOffset: 1_000, InputRoot: input.InputLeaf(0), InputLength: 1,
		MaterializationRoot: orderedMaterializationLeaf(substituted), MaterializationLength: 1,
		RejectionRoot: sha256.Sum256([]byte{2}), Claims: []namespace.Claim{substituted}}
	if _, err := input.VerifyClose(signedClaimClose(&forged), 0, forged); err == nil {
		t.Fatal("close accepted a reveal whose admission digest did not match its committed input")
	}
}
