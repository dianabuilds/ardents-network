package source

import (
	"crypto/ed25519"
	"crypto/tls"
	"encoding/hex"
	"testing"
)

func TestSourceTransportPinIndependentVector(t *testing.T) {
	// Independently computed with Python hashlib over the literal ASCII domain,
	// one zero byte and bytes 00..1f. Neither SPKI nor raw-key hashing matches.
	key := make(ed25519.PublicKey, 32)
	for i := range key {
		key[i] = byte(i)
	}
	pin, err := keyDigest(key)
	if err != nil || hex.EncodeToString(pin[:]) != "1ecf208aa34cb6c28a13a2304f1ef56a6c3377c26d7530fb91e12c85514856b8" {
		t.Fatal("Source transcript changed", pin, err)
	}
}

func TestCloneCertificateOwnsMutableInputs(t *testing.T) {
	input := tls.Certificate{
		Certificate:                  [][]byte{{1, 2, 3}},
		PrivateKey:                   ed25519.PrivateKey{4, 5, 6},
		SupportedSignatureAlgorithms: []tls.SignatureScheme{tls.Ed25519},
		OCSPStaple:                   []byte{7},
		SignedCertificateTimestamps:  [][]byte{{8}},
	}
	clone := cloneCertificate(input)
	input.Certificate[0][0] = 9
	input.PrivateKey.(ed25519.PrivateKey)[0] = 9
	input.SupportedSignatureAlgorithms[0] = tls.PSSWithSHA256
	input.OCSPStaple[0] = 9
	input.SignedCertificateTimestamps[0][0] = 9

	if clone.Certificate[0][0] != 1 || clone.PrivateKey.(ed25519.PrivateKey)[0] != 4 ||
		clone.SupportedSignatureAlgorithms[0] != tls.Ed25519 || clone.OCSPStaple[0] != 7 ||
		clone.SignedCertificateTimestamps[0][0] != 8 {
		t.Fatalf("certificate clone retained mutable input: %+v", clone)
	}
}
