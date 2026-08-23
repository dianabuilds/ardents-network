package route

import (
	"bytes"
	"crypto/ecdh"
	"crypto/hpke"
	"encoding/hex"
	"testing"
	"time"
)

func TestSealedIntroductionV1CanonicalVector(t *testing.T) {
	input := introductionFixture()
	input.Enc = bytes.Repeat([]byte{8}, encapsulationLength)
	input.Ciphertext = bytes.Repeat([]byte{9}, minimumCiphertext)
	raw, err := EncodeSealedIntroduction(input)
	if err != nil {
		t.Fatal(err)
	}
	const want = "617264656e74732d696e7465726163746976652d726f7574652d76310001440001021c617264656e74732d696e7465726163746976652d726f7574652d76310b00000000000000000000000000000000000000000000000000000000000000000000000000000d0c000000000000000000000000000000000000000000000000000000000000000e000000000000000000000000000000000000000000000000000000000000000f00000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000000000000684ee1801100000000000000000000000000000000000000000000000000000000000000120000000000000000000000000000000000000000000000000000000000000000200808080808080808080808080808080808080808080808080808080808080808001009090909090909090909090909090909"
	if hex.EncodeToString(raw) != want {
		t.Fatalf("canonical Introduction vector = %x, want %s", raw, want)
	}
	decoded, err := DecodeSealedIntroduction(raw)
	if err != nil || !equalIntroduction(decoded, input) {
		t.Fatalf("decoded Introduction = %+v, %v", decoded, err)
	}
}

func TestSealedIntroductionV1AuthenticatesItsVisibleContext(t *testing.T) {
	private, err := ecdh.X25519().NewPrivateKey(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	public, err := hpke.NewDHKEMPublicKey(private.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	recipient, err := hpke.NewDHKEMPrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	input, err := SealIntroduction(introductionFixture(), public, []byte("service-only material"))
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := OpenSealedIntroduction(input, recipient)
	if err != nil || string(plaintext) != "service-only material" {
		t.Fatalf("OpenSealedIntroduction = %q, %v", plaintext, err)
	}
	input.Digest[0] ^= 1
	if _, err := OpenSealedIntroduction(input, recipient); err == nil {
		t.Fatal("mutated visible context decrypted")
	}
}

func TestSealedIntroductionV1RejectsInvalidLengths(t *testing.T) {
	input := introductionFixture()
	input.Enc = bytes.Repeat([]byte{1}, encapsulationLength-1)
	input.Ciphertext = bytes.Repeat([]byte{2}, minimumCiphertext)
	if _, err := EncodeSealedIntroduction(input); err == nil {
		t.Fatal("short HPKE encapsulation was accepted")
	}
}

func introductionFixture() SealedIntroduction {
	return SealedIntroduction{NetworkID: identifier(11), Digest: identifier(12), Epoch: 13,
		IntroductionNodeID: identifier(14), RendezvousNodeID: identifier(15), Reachability: identifier(16),
		NotAfter: time.Unix(1_750_000_000, 0).UTC(), JoinHandle: identifier(17), EndpointHandshake: identifier(18)}
}
