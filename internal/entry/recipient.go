package entry

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

const recipientName = "recipient"

// entryRecipientCertificate opens the owner-local recipient identity. The
// Entry root creates it before any Invite can be imported, so an offline
// issuer can bind an Invite to the public key without receiving the private
// key. Reopening retains the exact key.
func entryRecipientCertificate(root string) (tls.Certificate, error) {
	seed, err := readBounded(filepath.Join(root, recipientName), ed25519.SeedSize)
	if os.IsNotExist(err) {
		seed = make([]byte, ed25519.SeedSize)
		if _, err = rand.Read(seed); err != nil {
			return tls.Certificate{}, err
		}
		if err = writeExclusive(filepath.Join(root, recipientName), seed); err != nil {
			return tls.Certificate{}, err
		}
		if err = syncDirectory(root); err != nil {
			return tls.Certificate{}, err
		}
	} else if err != nil || len(seed) != ed25519.SeedSize {
		return tls.Certificate{}, errors.New("entry recipient identity is invalid")
	}
	private := ed25519.NewKeyFromSeed(seed)
	public := private.Public().(ed25519.PublicKey)
	template := &x509.Certificate{SerialNumber: new(big.Int).SetBytes(public[:8]), Subject: pkix.Name{CommonName: "ardents-entry-recipient"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(16 * time.Minute), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		return tls.Certificate{}, err
	}
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: leaf}, nil
}

// RecipientPublicKey returns the exact public key an offline issuer must put
// in a recipient-bound v2 Invite.
func (owner *owner) RecipientPublicKey() ([32]byte, error) {
	if owner == nil || owner.recipient.Leaf == nil {
		return [32]byte{}, errors.New("entry recipient identity is unavailable")
	}
	public, ok := owner.recipient.Leaf.PublicKey.(ed25519.PublicKey)
	if !ok || len(public) != ed25519.PublicKeySize {
		return [32]byte{}, errors.New("entry recipient identity is invalid")
	}
	var result [32]byte
	copy(result[:], public)
	return result, nil
}

// RecipientCertificate returns a copy of the retained private TLS identity
// for the immediate native Entry attachment. It does not export key bytes.
func (owner *owner) RecipientCertificate() (tls.Certificate, error) {
	if owner == nil || owner.recipient.Leaf == nil || owner.recipient.PrivateKey == nil {
		return tls.Certificate{}, errors.New("entry recipient identity is unavailable")
	}
	copy := owner.recipient
	copy.Certificate = append([][]byte(nil), owner.recipient.Certificate...)
	return copy, nil
}
