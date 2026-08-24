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
	introduction, rendezvous, initiator, responder tls.Certificate
	introductionPublic, rendezvousPublic           [32]byte
	initiatorPublic, responderPublic               [32]byte
	hpkePrivate                                    hpke.PrivateKey
	hpkePublic                                     hpke.PublicKey
}

func material() (tracerMaterial, error) {
	introduction, introductionPublic, err := nodeCertificate("r105-introduction-tls", 1)
	if err != nil {
		return tracerMaterial{}, err
	}
	rendezvous, rendezvousPublic, err := nodeCertificate("r105-rendezvous-tls", 2)
	if err != nil {
		return tracerMaterial{}, err
	}
	initiator, initiatorPublic, err := nodeCertificate("r105-initiator-tls", 3)
	if err != nil {
		return tracerMaterial{}, err
	}
	responder, responderPublic, err := nodeCertificate("r105-responder-tls", 4)
	if err != nil {
		return tracerMaterial{}, err
	}
	hpkePrivate, hpkePublic, err := serviceHPKE()
	if err != nil {
		return tracerMaterial{}, err
	}
	return tracerMaterial{introduction: introduction, introductionPublic: introductionPublic, rendezvous: rendezvous,
		rendezvousPublic: rendezvousPublic, initiator: initiator, initiatorPublic: initiatorPublic,
		responder: responder, responderPublic: responderPublic, hpkePrivate: hpkePrivate, hpkePublic: hpkePublic}, nil
}

func nodeCertificate(label string, serial int64) (tls.Certificate, [32]byte, error) {
	seed := identifier(label)
	key := ed25519.NewKeyFromSeed(seed[:])
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: label},
		NotBefore: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), NotAfter: time.Date(2035, 1, 1, 0, 0, 0, 0, time.UTC),
		KeyUsage: x509.KeyUsageDigitalSignature, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		return tls.Certificate{}, [32]byte{}, err
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, [32]byte{}, err
	}
	var public [32]byte
	copy(public[:], key.Public().(ed25519.PublicKey))
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, public, nil
}

func serviceHPKE() (hpke.PrivateKey, hpke.PublicKey, error) {
	seed := identifier("r105-service-hpke")
	private, err := ecdh.X25519().NewPrivateKey(seed[:])
	if err != nil {
		return nil, nil, err
	}
	hpkePrivate, err := hpke.NewDHKEMPrivateKey(private)
	if err != nil {
		return nil, nil, err
	}
	hpkePublic, err := hpke.NewDHKEMPublicKey(private.PublicKey())
	return hpkePrivate, hpkePublic, err
}

func identifier(label string) [32]byte { return sha256.Sum256([]byte("r105-tracer-v1\x00" + label)) }
