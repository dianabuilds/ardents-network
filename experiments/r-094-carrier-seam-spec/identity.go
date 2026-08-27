//go:build ignore

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"time"
)

type identities struct {
	client, server, wrongServer tls.Certificate
	clientID, serverID          [32]byte
}

func newIdentities() (identities, error) {
	client, clientID, err := newIdentity("r094-client")
	if err != nil {
		return identities{}, err
	}
	server, serverID, err := newIdentity("r094-server")
	if err != nil {
		return identities{}, err
	}
	wrong, _, err := newIdentity("r094-wrong-server")
	if err != nil {
		return identities{}, err
	}
	return identities{client: client, server: server, wrongServer: wrong, clientID: clientID, serverID: serverID}, nil
}

// deterministicIdentities are public, experiment-only process fixtures. They
// let separate fault-lab processes derive the same non-production identities
// without writing private keys or certificates to the repository.
func deterministicIdentities() (identities, error) {
	client, clientID, err := deterministicIdentity("r094-fault-client", 1)
	if err != nil {
		return identities{}, err
	}
	server, serverID, err := deterministicIdentity("r094-fault-server", 2)
	if err != nil {
		return identities{}, err
	}
	wrong, _, err := deterministicIdentity("r094-fault-wrong-server", 3)
	if err != nil {
		return identities{}, err
	}
	return identities{client: client, server: server, wrongServer: wrong, clientID: clientID, serverID: serverID}, nil
}

func deterministicIdentity(name string, serialValue int64) (tls.Certificate, [32]byte, error) {
	seed := sha256.Sum256([]byte("public-disposable-" + name))
	private := ed25519.NewKeyFromSeed(seed[:])
	public := private.Public().(ed25519.PublicKey)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serialValue), Subject: pkix.Name{CommonName: name},
		NotBefore: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:  time.Date(2036, 1, 1, 0, 0, 0, 0, time.UTC),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		},
	}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		return tls.Certificate{}, [32]byte{}, err
	}
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		return tls.Certificate{}, [32]byte{}, err
	}
	var identity [32]byte
	copy(identity[:], public)
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: leaf}, identity, nil
}

func newIdentity(name string) (tls.Certificate, [32]byte, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return tls.Certificate{}, [32]byte{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, [32]byte{}, err
	}
	template := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: name},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		return tls.Certificate{}, [32]byte{}, err
	}
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		return tls.Certificate{}, [32]byte{}, err
	}
	var identity [32]byte
	copy(identity[:], public)
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: leaf}, identity, nil
}

func certificateIdentity(certificate tls.Certificate) ([32]byte, error) {
	leaf := certificate.Leaf
	if leaf == nil && len(certificate.Certificate) == 1 {
		var err error
		leaf, err = x509.ParseCertificate(certificate.Certificate[0])
		if err != nil {
			return [32]byte{}, err
		}
	}
	if leaf == nil {
		return [32]byte{}, errors.New("experiment certificate leaf is unavailable")
	}
	public, ok := leaf.PublicKey.(ed25519.PublicKey)
	if !ok || len(public) != ed25519.PublicKeySize {
		return [32]byte{}, errors.New("experiment certificate identity is invalid")
	}
	var result [32]byte
	copy(result[:], public)
	return result, nil
}

func clientTLS(certificate tls.Certificate, peer [32]byte) *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{certificate}, InsecureSkipVerify: true,
		SessionTicketsDisabled: true, NextProtos: []string{nativeProfile},
		VerifyConnection: exactPeer(peer),
	}
}

func serverTLS(certificate tls.Certificate, peer [32]byte) *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{certificate}, ClientAuth: tls.RequireAnyClientCert,
		SessionTicketsDisabled: true, NextProtos: []string{nativeProfile},
		VerifyConnection: exactPeer(peer),
	}
}

func exactPeer(expected [32]byte) func(tls.ConnectionState) error {
	return func(state tls.ConnectionState) error {
		if len(state.PeerCertificates) != 1 {
			return errUnauthorized
		}
		public, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
		if !ok || len(public) != ed25519.PublicKeySize || !bytes.Equal(public, expected[:]) {
			return errUnauthorized
		}
		return nil
	}
}
