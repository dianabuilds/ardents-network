package probe

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"errors"
	"time"
)

func cloneTLSMaterial(input *Config, identity ed25519.PublicKey, now time.Time) error {
	certificate := input.Certificate
	if len(certificate.Certificate) < 2 || len(certificate.Certificate) > 4 || certificate.PrivateKey == nil ||
		len(certificate.OCSPStaple) != 0 || len(certificate.SignedCertificateTimestamps) != 0 {
		return errors.New("node role-probe certificate chain is invalid")
	}
	cloned := make([][]byte, len(certificate.Certificate))
	parsed := make([]*x509.Certificate, len(certificate.Certificate))
	total := 0
	for index, raw := range certificate.Certificate {
		total += len(raw)
		if len(raw) == 0 || len(raw) > 64<<10 || total > 64<<10 {
			return errors.New("node role-probe certificate chain exceeds its bound")
		}
		cloned[index] = append([]byte(nil), raw...)
		var err error
		parsed[index], err = x509.ParseCertificate(cloned[index])
		if err != nil || now.Before(parsed[index].NotBefore) || !now.Before(parsed[index].NotAfter) {
			return errors.New("node role-probe certificate chain is invalid")
		}
	}
	leaf := parsed[0]
	public, publicOK := leafKey(leaf)
	private, privateOK := certificate.PrivateKey.(ed25519.PrivateKey)
	if !publicOK || !privateOK || len(private) != ed25519.PrivateKeySize ||
		!bytes.Equal(public, private.Public().(ed25519.PublicKey)) || !supportsServerAuth(leaf) ||
		!now.Add(input.MaximumDuty).Before(leaf.NotAfter) {
		return errors.New("node role-probe certificate key or validity is invalid")
	}
	for index := 0; index < len(parsed)-1; index++ {
		if !parsed[index+1].IsCA || parsed[index].CheckSignatureFrom(parsed[index+1]) != nil {
			return errors.New("node role-probe certificate chain is invalid")
		}
	}
	root := parsed[len(parsed)-1]
	if !root.IsCA || root.CheckSignatureFrom(root) != nil {
		return errors.New("node role-probe certificate root is invalid")
	}
	if bytes.Equal(public, identity) {
		return errors.New("node identity and role-probe transport keys must be separate")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(input.ClientRootPEM) {
		return errors.New("node role-probe client roots are invalid")
	}
	certificate.Certificate = cloned
	certificate.PrivateKey = append(ed25519.PrivateKey(nil), private...)
	certificate.Leaf = nil
	input.Certificate = certificate
	input.ClientRootPEM = append([]byte(nil), input.ClientRootPEM...)
	input.ClientKeyPins = append([][32]byte(nil), input.ClientKeyPins...)
	return nil
}

func leafKey(leaf *x509.Certificate) (ed25519.PublicKey, bool) {
	public, ok := leaf.PublicKey.(ed25519.PublicKey)
	return public, ok
}

func supportsServerAuth(leaf *x509.Certificate) bool {
	for _, usage := range leaf.ExtKeyUsage {
		if usage == x509.ExtKeyUsageServerAuth {
			return true
		}
	}
	return false
}
