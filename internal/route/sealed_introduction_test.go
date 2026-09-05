package route

import (
	"bytes"
	"crypto/ecdh"
	"crypto/hpke"
	"encoding/hex"
	"testing"
	"time"
)

func TestSealedIntroductionV2CanonicalVector(t *testing.T) {
	input := introductionFixture()
	input.Enc = bytes.Repeat([]byte{8}, encapsulationLength)
	input.Ciphertext = bytes.Repeat([]byte{9}, minimumCiphertext)
	raw, err := EncodeSealedIntroduction(input)
	if err != nil {
		t.Fatal(err)
	}
	const want = "617264656e74732d696e7465726163746976652d726f7574652d76320001440002031c617264656e74732d696e7465726163746976652d726f7574652d76320b00000000000000000000000000000000000000000000000000000000000000000000000000000d0c000000000000000000000000000000000000000000000000000000000000000e000000000000000000000000000000000000000000000000000000000000000f00000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000000000000684ee1801100000000000000000000000000000000000000000000000000000000000000120000000000000000000000000000000000000000000000000000000000000000200808080808080808080808080808080808080808080808080808080808080808001009090909090909090909090909090909"
	if hex.EncodeToString(raw) != want {
		t.Fatalf("canonical Introduction vector = %x, want %s", raw, want)
	}
	decoded, err := DecodeSealedIntroduction(raw)
	if err != nil || !equalIntroduction(decoded, input) {
		t.Fatalf("decoded Introduction = %+v, %v", decoded, err)
	}
}

func TestSealedIntroductionV2AuthenticatesItsVisibleContext(t *testing.T) {
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

func TestSealedIntroductionV2RejectsV1KnownAnswer(t *testing.T) {
	const wire = "617264656e74732d696e7465726163746976652d726f7574652d76310001590001031c617264656e74732d696e7465726163746976652d726f7574652d76310b00000000000000000000000000000000000000000000000000000000000000000000000000000d0c000000000000000000000000000000000000000000000000000000000000000e000000000000000000000000000000000000000000000000000000000000000f00000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000000000000684ee18011000000000000000000000000000000000000000000000000000000000000001200000000000000000000000000000000000000000000000000000000000000002045111aa82f358cf4071b9000447b26c1f52cb51c4981251355f3a9140db0bf51002544bb7ed1eaac09ae913167549c58926c8bf4616257bcf0db2a10ec5137782f8064c4a00c82"
	raw, err := hex.DecodeString(wire)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSealedIntroduction(raw); err == nil {
		t.Fatal("Route v1 sealed Introduction known answer was accepted by Route v2")
	}
}

func TestSealedIntroductionV2RefusesEveryContextSubstitution(t *testing.T) {
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
	mutations := map[string]func(*SealedIntroduction){
		"network":       func(value *SealedIntroduction) { value.NetworkID[0]++ },
		"epoch":         func(value *SealedIntroduction) { value.Epoch++ },
		"digest":        func(value *SealedIntroduction) { value.Digest[0]++ },
		"introduction":  func(value *SealedIntroduction) { value.IntroductionNodeID[0]++ },
		"rendezvous":    func(value *SealedIntroduction) { value.RendezvousNodeID[0]++ },
		"reachability":  func(value *SealedIntroduction) { value.Reachability[0]++ },
		"expiry":        func(value *SealedIntroduction) { value.NotAfter = value.NotAfter.Add(time.Second) },
		"join":          func(value *SealedIntroduction) { value.JoinHandle[0]++ },
		"handshake":     func(value *SealedIntroduction) { value.EndpointHandshake[0]++ },
		"encapsulation": func(value *SealedIntroduction) { value.Enc[0]++ },
		"ciphertext":    func(value *SealedIntroduction) { value.Ciphertext[0]++ },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := input
			changed.Enc = append([]byte(nil), input.Enc...)
			changed.Ciphertext = append([]byte(nil), input.Ciphertext...)
			mutate(&changed)
			if _, err := OpenSealedIntroduction(changed, recipient); err == nil {
				t.Fatal("substituted sealed Introduction decrypted")
			}
		})
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

func TestSealedIntroductionV1RefusesFractionalExpiry(t *testing.T) {
	input := introductionFixture()
	input.NotAfter = input.NotAfter.Add(time.Nanosecond)
	if _, err := EncodeSealedIntroduction(input); err == nil {
		t.Fatal("fractional expiry was accepted")
	}
}

func introductionFixture() SealedIntroduction {
	return SealedIntroduction{NetworkID: identifier(11), Digest: identifier(12), Epoch: 13,
		IntroductionNodeID: identifier(14), RendezvousNodeID: identifier(15), Reachability: identifier(16),
		NotAfter: time.Unix(1_750_000_000, 0).UTC(), JoinHandle: identifier(17), EndpointHandshake: identifier(18)}
}
