package fixture

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"time"
)

type nodeCredential struct {
	certificate []byte
	key         []byte
	root        []byte
	private     ed25519.PrivateKey
	pin         [32]byte
	sourcePin   [32]byte
	leaf        *x509.Certificate
}

func newNodeAuthority(name string, now time.Time) (nodeCredential, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nodeCredential{}, err
	}
	template := &x509.Certificate{SerialNumber: nodeSerial(now), Subject: pkix.Name{CommonName: name},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(72 * time.Hour), IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		return nodeCredential{}, err
	}
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		return nodeCredential{}, err
	}
	return nodeCredential{root: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw}),
		private: private, leaf: leaf}, nil
}

func newNodeLeaf(authority nodeCredential, name string, usage x509.ExtKeyUsage, now time.Time) (nodeCredential, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nodeCredential{}, err
	}
	template := &x509.Certificate{SerialNumber: nodeSerial(now), Subject: pkix.Name{CommonName: name},
		DNSNames: []string{name}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(72 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
	raw, err := x509.CreateCertificate(rand.Reader, template, authority.leaf, public, authority.private)
	if err != nil {
		return nodeCredential{}, err
	}
	keyRaw, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return nodeCredential{}, err
	}
	pinRaw, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		return nodeCredential{}, err
	}
	sourceRaw := append([]byte("ardents-h3-source-transport-key-v1\x00"), public...)
	certificate := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw}), authority.root...)
	return nodeCredential{certificate: certificate, key: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyRaw}),
		root: authority.root, private: private, pin: sha256.Sum256(pinRaw), sourcePin: sha256.Sum256(sourceRaw)}, nil
}

func nodePrivatePEM(private ed25519.PrivateKey) ([]byte, error) {
	raw, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, errors.New("node private key encoding is empty")
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: raw}), nil
}

func nodeSerial(now time.Time) *big.Int {
	serial := big.NewInt(now.UnixNano())
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err == nil {
		serial.Lsh(serial, 32).Or(serial, new(big.Int).SetBytes(suffix[:]))
	}
	return serial
}
