package namespace_test

import (
	"crypto/ed25519"
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
}
