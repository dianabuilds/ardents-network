package namespace_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/admission"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/claim"
)

func TestClaimCommitmentAdmissionIsSpentAndRevealDerivesItsDigest(t *testing.T) {
	t.Parallel()
	network := [32]byte{11}
	gate, err := admission.NewAdmission([32]byte{1}, network, 7, [32]byte{2})
	if err != nil {
		t.Fatal(err)
	}
	key := deterministicAuthority("claim-ingestion")
	submission := claim.Claim{Name: "alice", Secret: [32]byte{3}}
	copy(submission.Authority[:], key.Public().(ed25519.PublicKey))
	commitment := claim.CommitmentFor(network, 7, submission)
	challenge, err := gate.Issue(100, "root-claim", commitment, [32]byte{4}, 1_000, [16]byte{5})
	if err != nil {
		t.Fatal(err)
	}
	proof, _ := challenge.Solve()
	accepted, err := claim.AdmitClaimCommitment(gate, 100, commitment, proof)
	if err != nil {
		t.Fatal(err)
	}
	copy(submission.Signature[:], ed25519.Sign(key, claim.RevealTranscript(network, 7, claim.Claim{Commitment: commitment})))
	reveal, err := accepted.Reveal(submission.Name, submission.Secret, submission.Authority, submission.Signature)
	if err != nil || reveal.AdmissionDigest == [32]byte{} {
		t.Fatalf("reveal=%+v err=%v", reveal, err)
	}
	if _, err := claim.AdmitClaimCommitment(gate, 100, commitment, proof); err == nil {
		t.Fatal("replayed proof admitted another commitment")
	}
	if _, err := accepted.Reveal("bob", submission.Secret, submission.Authority, submission.Signature); err == nil {
		t.Fatal("signature opened a different Name")
	}
	input, err := accepted.EpochInput()
	if err != nil || input.Commitment() != commitment || len(input.Canonical()) != 64 ||
		input.InputLeaf(0) != orderedInputLeaf(reveal) {
		t.Fatalf("Epoch input=%x commitment=%x err=%v", input.Canonical(), input.Commitment(), err)
	}
	close := claim.ClaimProof{Network: network, Epoch: 7, Rule: "ardents-name-claim-order-v1",
		CutoffOffset: 1_000, InputRoot: input.InputLeaf(0), InputLength: 1,
		MaterializationRoot: orderedMaterializationLeaf(reveal), MaterializationLength: 1,
		RejectionRoot: sha256.Sum256([]byte{2}), Claims: []claim.Claim{reveal}}
	if _, err := (claim.EpochClaimInput{}).VerifyClose(claim.ClaimOrder{}, 0, close); err == nil {
		t.Fatal("caller-built zero Epoch input verified a close")
	}
	if _, err := input.VerifyClose(signedClaimClose(&close), 0, close); err != nil {
		t.Fatalf("admitted input did not yield verified winner: %v", err)
	}
	substituted := reveal
	substituted.AdmissionDigest = [32]byte{9}
	forged := claim.ClaimProof{Network: network, Epoch: 7, Rule: "ardents-name-claim-order-v1",
		CutoffOffset: 1_000, InputRoot: input.InputLeaf(0), InputLength: 1,
		MaterializationRoot: orderedMaterializationLeaf(substituted), MaterializationLength: 1,
		RejectionRoot: sha256.Sum256([]byte{2}), Claims: []claim.Claim{substituted}}
	if _, err := input.VerifyClose(signedClaimClose(&forged), 0, forged); err == nil {
		t.Fatal("close accepted a reveal whose admission digest did not match its committed input")
	}
}
