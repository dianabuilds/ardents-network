//go:build linux || darwin

package provisioning

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"io"
	"math/big"

	"github.com/dianabuilds/ardents-network/internal/network/epoch"
)

// Exact Node transport leaf profile from the current TLS material owner. This
// self-signature is not Network membership, State trust or process admission.
// Forwarding, JOIN and Descriptor share this pure codec; each role command
// supplies its own prepared Record facts and freshly born Node key.
func createNodeCertificate(facts epoch.InitialClosedRecord, key ed25519.PrivateKey) ([]byte, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	if serial.Sign() <= 0 {
		return nil, artifacts.ErrInput
	}
	return createNodeCertificateWithSerial(facts, key, rand.Reader, serial)
}

func createNodeCertificateWithSerial(facts epoch.InitialClosedRecord, key ed25519.PrivateKey, random io.Reader, serial *big.Int) ([]byte, error) {
	leaf := &x509.Certificate{SerialNumber: serial, NotBefore: facts.ValidFrom, NotAfter: facts.ValidUntil, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(random, leaf, leaf, facts.PublicKey, key)
	if err != nil {
		return nil, err
	}
	if err := verifyNodeCertificate(der, facts); err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil
}

func verifyNodeCertificate(der []byte, facts epoch.InitialClosedRecord) error {
	c, err := x509.ParseCertificate(der)
	if err != nil {
		return artifacts.ErrInput
	}
	public, ok := c.PublicKey.(ed25519.PublicKey)
	if !ok || !bytes.Equal(public, facts.PublicKey) || c.SignatureAlgorithm != x509.PureEd25519 || c.CheckSignature(c.SignatureAlgorithm, c.RawTBSCertificate, c.Signature) != nil || !c.NotBefore.Equal(facts.ValidFrom) || !c.NotAfter.Equal(facts.ValidUntil) || c.SerialNumber.Sign() <= 0 || c.SerialNumber.BitLen() > 128 || c.Version != 3 || c.IsCA || !c.BasicConstraintsValid || c.MaxPathLen != -1 || c.KeyUsage != x509.KeyUsageDigitalSignature || len(c.ExtKeyUsage) != 2 || c.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth || c.ExtKeyUsage[1] != x509.ExtKeyUsageClientAuth || len(c.UnknownExtKeyUsage) != 0 || len(c.UnhandledCriticalExtensions) != 0 || !bytes.Equal(c.RawSubject, []byte{0x30, 0}) || !bytes.Equal(c.RawIssuer, c.RawSubject) || len(c.Extensions) != 3 {
		return artifacts.ErrInput
	}
	seen := map[string]bool{}
	for _, extension := range c.Extensions {
		oid := extension.Id.String()
		if seen[oid] || oid != "2.5.29.15" && oid != "2.5.29.37" && oid != "2.5.29.19" {
			return artifacts.ErrInput
		}
		seen[oid] = true
	}
	return nil
}
