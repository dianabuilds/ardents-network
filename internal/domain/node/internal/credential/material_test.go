package credential

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"testing"
)

func TestDecodeCredentialRetainsAndClearsRuntimeKey(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(private)
	template := &x509.Certificate{SerialNumber: big.NewInt(1)}
	cert, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(encoded)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})
	defer clear(keyPEM)

	material, err := decodeCredential(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	if !private.Equal(material.key) {
		t.Fatal("decoded credential did not retain the original key")
	}
	if err := material.Close(); err != nil {
		t.Fatal(err)
	}
	if material.certificate.PrivateKey != nil || !material.key.Equal(ed25519.PrivateKey(make([]byte, ed25519.PrivateKeySize))) {
		t.Fatal("closed credential retained runtime key material")
	}
}
