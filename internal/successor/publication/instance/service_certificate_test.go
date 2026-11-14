package instance

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"math/big"
	"testing"
	"time"
)

// The Service certificate is rebuilt from an immutable binding credential on
// every attachment. PureEd25519 signing is deterministic, so the DER must be
// byte-identical for the same template and key — the precondition for caching
// it per binding (audit DD-7).
func TestServiceCertificateDERIsDeterministic(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	notBefore := time.Unix(1_800_000_000, 0).UTC()
	template := &x509.Certificate{SerialNumber: big.NewInt(7), NotBefore: notBefore, NotAfter: notBefore.Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, SignatureAlgorithm: x509.PureEd25519}
	first, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	second, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("PureEd25519 certificate DER is not deterministic; caching would be unsafe")
	}
}
