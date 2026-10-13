package wire

import (
	"crypto/ed25519"
	"crypto/tls"
	"testing"
)

func TestInitialResponseRequiresExactProofContinuityAndOneUse(t *testing.T) {
	t.Parallel()
	// Private TLS-object fixture isolates response mechanism, not authority.
	binding := &TLSBinding{connection: &tls.Conn{}, key: [32]byte{1}, exporter: [32]byte{2}}
	expected := Challenge{Network: [32]byte{3}, Target: [32]byte{4}, InstanceGeneration: 5, Context: [32]byte{6}}
	challenge := expected
	challenge.Nonce = [32]byte{7}
	initiator, err := NewContinuity(binding.key, RoleInitiator, 1, 0, 0, 0, expected.Context, binding.exporter)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := VerifyInitialRequest(binding, expected, challenge, initiator)
	if err != nil {
		t.Fatal(err)
	}
	private := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	defer clear(private)
	var instance [32]byte
	copy(instance[:], private.Public().(ed25519.PublicKey))
	digest, err := ChallengeDigest(challenge)
	if err != nil {
		t.Fatal(err)
	}
	proof := Proof{ChallengeDigest: digest}
	copy(proof.Signature[:], ed25519.Sign(private, digest[:]))
	publisher, err := NewContinuity(binding.key, RolePublisher, 1, 0, 0, 0, expected.Context, binding.exporter)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewInitialResponse(receipt, instance, proof, publisher); err == nil {
		t.Fatal("unconsumed request supplied response")
	}
	if _, err := receipt.Consume(binding, expected); err != nil {
		t.Fatal(err)
	}
	foreignProof := proof
	foreignProof.ChallengeDigest[0]++
	if _, err := NewInitialResponse(receipt, instance, foreignProof, publisher); err == nil {
		t.Fatal("foreign challenge response accepted")
	}
	foreignProof = proof
	foreignProof.Signature[0]++
	if _, err := NewInitialResponse(receipt, instance, foreignProof, publisher); err == nil {
		t.Fatal("altered signature response accepted")
	}
	if _, err := NewInitialResponse(receipt, instance, proof, initiator); err == nil {
		t.Fatal("wrong role response accepted")
	}
	advanced, err := NewContinuity(binding.key, RolePublisher, 1, 0, 1, 0, expected.Context, binding.exporter)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewInitialResponse(receipt, instance, proof, advanced); err == nil {
		t.Fatal("valid MAC with nonzero initial offsets accepted")
	}
	response, err := NewInitialResponse(receipt, instance, proof, publisher)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewInitialResponse(receipt, instance, proof, publisher); err == nil {
		t.Fatal("same request supplied two responses")
	}
	if _, err := response.Consume(&TLSBinding{}, proof); err == nil {
		t.Fatal("foreign TLS consumed response")
	}
	copied := &InitialResponse{self: response.self, binding: response.binding, proof: response.proof, body: response.body}
	if _, err := copied.Consume(binding, proof); err == nil {
		t.Fatal("copied response consumed")
	}
	if raw, err := response.Consume(binding, proof); err != nil || len(raw) == 0 {
		t.Fatal("original response refused", err)
	}
	if _, err := response.Consume(binding, proof); err == nil {
		t.Fatal("response emitted twice")
	}
}
