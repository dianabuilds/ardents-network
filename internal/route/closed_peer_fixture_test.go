package route

// Shared peer-identity fixtures for the maintained closed Carrier and
// retained sealed Introduction tests. entryBindingCertificate,
// identifierFromKey, identifier, and equalIntroduction were relocated from
// the retired generation-2 EntryBinding and LegBinding test files when
// ADR-0093 deleted those grammars; the closed Carrier tests still need exact
// self-signed peer certificates and deterministic 32-byte identifiers.

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"
)

func entryBindingCertificate(t *testing.T, serial int64) tls.Certificate {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{byte(serial)}, ed25519.SeedSize))
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "route.test"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Minute), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, private.Public(), private)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: leaf}
}

func identifierFromKey(key ed25519.PublicKey) [32]byte {
	var result [32]byte
	copy(result[:], key)
	return result
}

func identifier(value byte) [32]byte {
	return [32]byte{value}
}

func equalIntroduction(left, right SealedIntroduction) bool {
	return left.NetworkID == right.NetworkID && left.Digest == right.Digest && left.Epoch == right.Epoch &&
		left.IntroductionNodeID == right.IntroductionNodeID && left.RendezvousNodeID == right.RendezvousNodeID &&
		left.Reachability == right.Reachability && left.NotAfter.Equal(right.NotAfter) && left.JoinHandle == right.JoinHandle &&
		left.EndpointHandshake == right.EndpointHandshake && bytes.Equal(left.Enc, right.Enc) && bytes.Equal(left.Ciphertext, right.Ciphertext)
}
