package routesmoke

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"
)

type identity struct {
	private          ed25519.PrivateKey
	public           [32]byte
	certificate, key []byte
}

func newIdentity(now time.Time) (identity, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return identity{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		return identity{}, err
	}
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "route.smoke"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(2 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		return identity{}, err
	}
	key, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return identity{}, err
	}
	var fixed [32]byte
	copy(fixed[:], public)
	return identity{private: private, public: fixed,
		certificate: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw}),
		key:         pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})}, nil
}
