package wire

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"testing"
)

// This fixture isolates transcript/signature and one-use receipt mechanics.
// Its incomplete TLS object supplies no genuine transport or Service authority.
func TestPeerInitialReceiptRequiresExactResponseAndSingleConsumption(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var instance [32]byte
	copy(instance[:], public)
	binding := &TLSBinding{connection: &tls.Conn{}, key: [32]byte{1}, exporter: [32]byte{2}}
	challenge := Challenge{Network: [32]byte{3}, Target: [32]byte{4}, Context: [32]byte{5}, Nonce: [32]byte{6}, InstanceGeneration: 7}
	digest, err := ChallengeDigest(challenge)
	if err != nil {
		t.Fatal(err)
	}
	proof := Proof{ChallengeDigest: digest}
	copy(proof.Signature[:], ed25519.Sign(private, digest[:]))
	request, err := NewContinuity(binding.key, RoleInitiator, 1, 0, 0, 0, challenge.Context, binding.exporter)
	if err != nil {
		t.Fatal(err)
	}
	response, err := NewContinuity(binding.key, RolePublisher, 1, 0, 0, 0, challenge.Context, binding.exporter)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := VerifyPeerInitialResponse(binding, instance, challenge, request, proof, response)
	if err != nil {
		t.Fatal(err)
	}
	foreign := &TLSBinding{connection: binding.connection, key: binding.key, exporter: binding.exporter}
	if err := receipt.Consume(foreign, challenge); err == nil {
		t.Fatal("foreign TLS receipt consumption accepted")
	}
	copy := &PeerInitialReceipt{self: receipt, binding: binding, challenge: challenge}
	if err := copy.Consume(binding, challenge); err == nil {
		t.Fatal("copied receipt supplied initial state")
	}
	if err := receipt.Consume(binding, challenge); err != nil {
		t.Fatal(err)
	}
	if err := receipt.Consume(binding, challenge); err == nil {
		t.Fatal("initial state consumed twice")
	}
	wrongProof := proof
	wrongProof.Signature[0] ^= 1
	if _, err := VerifyPeerInitialResponse(binding, instance, challenge, request, wrongProof, response); err == nil {
		t.Fatal("changed Instance proof accepted")
	}
	nonzero, err := NewContinuity(binding.key, RolePublisher, 1, 0, 1, 0, challenge.Context, binding.exporter)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyPeerInitialResponse(binding, instance, challenge, request, proof, nonzero); err == nil {
		t.Fatal("valid MAC with nonzero initial offsets accepted")
	}
	wrongGeneration, err := NewContinuity(binding.key, RolePublisher, 2, 0, 0, 0, challenge.Context, binding.exporter)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyPeerInitialResponse(binding, instance, challenge, request, proof, wrongGeneration); err == nil {
		t.Fatal("fresh generation substituted for initial state")
	}
	binding.Erase()
	if _, err := VerifyPeerInitialResponse(binding, instance, challenge, request, proof, response); err == nil {
		t.Fatal("retired TLS supplied response state")
	}
}
