//go:build ignore

package main

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"time"
)

type tracerMaterial struct {
	introduction       tls.Certificate
	introductionPublic [32]byte
	hpkePrivate        hpke.PrivateKey
	hpkePublic         hpke.PublicKey
}

func material() (tracerMaterial, error) {
	seed := identifier("r105-introduction-tls")
	key := ed25519.NewKeyFromSeed(seed[:])
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "r105-introduction"},
		NotBefore: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), NotAfter: time.Date(2035, 1, 1, 0, 0, 0, 0, time.UTC),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		return tracerMaterial{}, err
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return tracerMaterial{}, err
	}
	hpkeSeed := identifier("r105-service-hpke")
	private, err := ecdh.X25519().NewPrivateKey(hpkeSeed[:])
	if err != nil {
		return tracerMaterial{}, err
	}
	hpkePrivate, err := hpke.NewDHKEMPrivateKey(private)
	if err != nil {
		return tracerMaterial{}, err
	}
	hpkePublic, err := hpke.NewDHKEMPublicKey(private.PublicKey())
	if err != nil {
		return tracerMaterial{}, err
	}
	var public [32]byte
	copy(public[:], key.Public().(ed25519.PublicKey))
	return tracerMaterial{introduction: tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf},
		introductionPublic: public, hpkePrivate: hpkePrivate, hpkePublic: hpkePublic}, nil
}

func identifier(label string) [32]byte { return sha256.Sum256([]byte("r105-tracer-v1\x00" + label)) }
