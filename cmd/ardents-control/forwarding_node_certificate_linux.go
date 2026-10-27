package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"

	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
)

// Exact Node transport leaf profile from the current TLS material owner. This
// self-signature is not Network membership, State trust or process admission.
func createForwardingNodeCertificate(facts epoch.InitialClosedRecord, key ed25519.PrivateKey) ([]byte, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	if serial.Sign() <= 0 {
		return nil, errSourceControlInput
	}
	leaf := &x509.Certificate{SerialNumber: serial, NotBefore: facts.ValidFrom, NotAfter: facts.ValidUntil, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, leaf, leaf, facts.PublicKey, key)
	if err != nil {
		return nil, err
	}
	if err := verifyForwardingNodeCertificate(der, facts); err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil
}

func verifyForwardingNodeCertificate(der []byte, facts epoch.InitialClosedRecord) error {
	c, err := x509.ParseCertificate(der)
	if err != nil {
		return errSourceControlInput
	}
	public, ok := c.PublicKey.(ed25519.PublicKey)
	if !ok || !bytes.Equal(public, facts.PublicKey) || c.SignatureAlgorithm != x509.PureEd25519 || c.CheckSignature(c.SignatureAlgorithm, c.RawTBSCertificate, c.Signature) != nil || !c.NotBefore.Equal(facts.ValidFrom) || !c.NotAfter.Equal(facts.ValidUntil) || c.SerialNumber.Sign() <= 0 || c.SerialNumber.BitLen() > 128 || c.Version != 3 || c.IsCA || !c.BasicConstraintsValid || c.MaxPathLen != -1 || c.KeyUsage != x509.KeyUsageDigitalSignature || len(c.ExtKeyUsage) != 2 || c.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth || c.ExtKeyUsage[1] != x509.ExtKeyUsageClientAuth || len(c.UnknownExtKeyUsage) != 0 || len(c.UnhandledCriticalExtensions) != 0 || !bytes.Equal(c.RawSubject, []byte{0x30, 0}) || !bytes.Equal(c.RawIssuer, c.RawSubject) || len(c.Extensions) != 3 {
		return errSourceControlInput
	}
	seen := map[string]bool{}
	for _, extension := range c.Extensions {
		oid := extension.Id.String()
		if seen[oid] || oid != "2.5.29.15" && oid != "2.5.29.37" && oid != "2.5.29.19" {
			return errSourceControlInput
		}
		seen[oid] = true
	}
	return nil
}
