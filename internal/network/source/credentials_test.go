package source

import (
	"crypto/ed25519"
	"crypto/tls"
	"testing"
)

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
