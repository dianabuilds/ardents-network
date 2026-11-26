package wire

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"testing"
)

func TestInstanceProofBindsIndependentCanonicalChallenge(t *testing.T) {
	t.Parallel()
	// Fixture keys sign an independently encoded wire digest. They grant no
	// Publication/Execution/Route authority and do not stand in for its lease.
	private := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	defer clear(private)
	var expected [32]byte
	copy(expected[:], private.Public().(ed25519.PublicKey))
	challenge := Challenge{Network: [32]byte{1}, Target: [32]byte{2}, InstanceGeneration: 3,
		Context: [32]byte{4}, Nonce: [32]byte{5}}
	payload := append(append([]byte(nil), challenge.Network[:]...), challenge.Target[:]...)
	payload = binary.BigEndian.AppendUint64(payload, 3)
	payload = append(payload, challenge.Context[:]...)
	payload = append(payload, challenge.Nonce[:]...)
	// This oracle spells the fixed Challenge envelope independently of the codec.
	body := append([]byte{0, 3, 1, 37}, "ardents-service-connection-profile-v3"...)
	body = append(body, payload...)
	envelope := binary.BigEndian.AppendUint16([]byte("ardents-service-connection-v3\x00"), uint16(len(body)))
	digest := sha256.Sum256(append(envelope, body...))
	proof := Proof{ChallengeDigest: digest}
	copy(proof.Signature[:], ed25519.Sign(private, digest[:]))
	if err := verifyInstanceProof(expected, challenge, proof); err != nil {
		t.Fatalf("independent canonical signature refused: %v", err)
	}
	for _, change := range []struct {
		name string
		edit func(*Challenge)
	}{
		{"Network", func(c *Challenge) { c.Network[0]++ }},
		{"Target", func(c *Challenge) { c.Target[0]++ }},
		{"generation", func(c *Challenge) { c.InstanceGeneration++ }},
		{"Attachment context", func(c *Challenge) { c.Context[0]++ }},
		{"nonce", func(c *Challenge) { c.Nonce[0]++ }},
	} {
		t.Run(change.name, func(t *testing.T) {
			foreign := challenge
			change.edit(&foreign)
			if err := verifyInstanceProof(expected, foreign, proof); err == nil {
				t.Fatalf("proof from different pending challenge accepted: %v", err)
			}
		})
	}
	for _, change := range []struct {
		name string
		edit func(*Proof)
	}{
		{"digest", func(p *Proof) { p.ChallengeDigest[0]++ }},
		{"signature", func(p *Proof) { p.Signature[0] ^= 1 }},
		{"zero signature", func(p *Proof) { p.Signature = [64]byte{} }},
	} {
		t.Run(change.name, func(t *testing.T) {
			foreign := proof
			change.edit(&foreign)
			if err := verifyInstanceProof(expected, challenge, foreign); err == nil {
				t.Fatalf("altered protocol proof accepted: %v", err)
			}
		})
	}
	foreignKey := expected
	foreignKey[0] ^= 1
	for _, key := range [][32]byte{{}, foreignKey} {
		if err := verifyInstanceProof(key, challenge, proof); err == nil {
			t.Fatal("absent or foreign expected Instance key accepted")
		}
	}
	// The peer has a genuine signature from this fixture key, but over different
	// bytes. Checking only its self-supplied digest would accept this substitution.
	foreignDigest := sha256.Sum256([]byte("another signed challenge"))
	foreignProof := Proof{ChallengeDigest: foreignDigest}
	copy(foreignProof.Signature[:], ed25519.Sign(private, foreignDigest[:]))
	if err := verifyInstanceProof(expected, challenge, foreignProof); err == nil {
		t.Fatal("valid signature over peer-selected digest replaced pending Challenge")
	}
	invalid := challenge
	invalid.Nonce = [32]byte{}
	if err := verifyInstanceProof(expected, invalid, proof); err == nil {
		t.Fatal("invalid pending challenge accepted")
	}
}
