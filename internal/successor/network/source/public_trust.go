//go:build linux

package source

import (
	"bytes"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"time"
)

// PublicSource binds an externally supplied public chain to its declared trust.
type PublicSource struct {
	Source         Source
	CertificatePEM []byte
}

// PublicTrust is an offline observation of exactly two Sources and one client.
// It contains no private material, clock callback, live Plan or State authority.
type PublicTrust struct {
	Sources              [2]PublicSource
	ClientCertificatePEM []byte
	ClientRootPEM        []byte
	ClientKeyDigest      [32]byte
}

// ObservePublicTrust checks public credential purpose, explicit CA/name/pin and
// the same collision/declaration rules used by New. Reference is solely the
// caller's offline observation; runtime State independently owns handshake time.
func ObservePublicTrust(input PublicTrust, authorities map[[32]byte]ed25519.PublicKey, reference time.Time) error {
	if reference.IsZero() || reference.Nanosecond() != 0 {
		return errors.New("source observation reference is invalid")
	}
	client, err := observeChain(input.ClientCertificatePEM, input.ClientRootPEM, "", x509.ExtKeyUsageClientAuth, reference)
	if err != nil {
		return err
	}
	pin, err := certificateDigest(client)
	if err != nil || pin != input.ClientKeyDigest {
		return errors.New("source client observation pin differs")
	}
	config := Config{ClientCertificate: client}
	for i, declared := range input.Sources {
		certificate, err := observeChain(declared.CertificatePEM, declared.Source.RootPEM, declared.Source.ServerName, x509.ExtKeyUsageServerAuth, reference)
		if err != nil {
			return err
		}
		pin, err := certificateDigest(certificate)
		if err != nil || pin != declared.Source.LeafKeyDigest {
			return errors.New("source observation pin differs")
		}
		config.Sources[i] = declared.Source
		_, err = configureServerTrust(Config{ServeAddress: declared.Source.Address, ServeCertificate: certificate, ServeClientRootPEM: input.ClientRootPEM, ServeClientKeyDigests: [][32]byte{input.ClientKeyDigest}}, authorities, true)
		if err != nil {
			return err
		}
	}
	return configureClientTrust(&Plan{}, config, authorities, true)
}

// Only certificate PEM blocks are accepted. Mixed/private/trailing material is
// rejected rather than ignored by AppendCertsFromPEM. Roots are always explicit.
func publicCertificates(raw []byte) ([]*x509.Certificate, error) {
	if len(raw) == 0 || len(raw) > 64<<10 {
		return nil, errors.New("source public PEM bound is invalid")
	}
	var certificates []*x509.Certificate
	for len(bytes.TrimSpace(raw)) != 0 {
		raw = bytes.TrimSpace(raw)
		if !bytes.HasPrefix(raw, []byte("-----BEGIN CERTIFICATE-----")) {
			return nil, errors.New("source public PEM is mixed or invalid")
		}
		// Decode only the first complete block. pem.Decode may otherwise skip
		// a malformed BEGIN and accept a later block, silently ignoring bytes.
		footer := []byte("-----END CERTIFICATE-----")
		end := bytes.Index(raw, footer)
		if end < 0 {
			return nil, errors.New("source public chain is incomplete")
		}
		end += len(footer)
		encoded := raw[:end]
		if bytes.Count(encoded, []byte("-----BEGIN ")) != 1 {
			return nil, errors.New("source public PEM contains ignored material")
		}
		block, rest := pem.Decode(encoded)
		if block == nil || len(bytes.TrimSpace(rest)) != 0 || block.Type != "CERTIFICATE" || len(block.Headers) != 0 || len(certificates) == 4 {
			return nil, errors.New("source public chain is invalid")
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, errors.New("source public certificate is invalid")
		}
		certificates = append(certificates, certificate)
		raw = raw[end:]
	}
	return certificates, nil
}

func observeChain(chainPEM, rootPEM []byte, name string, usage x509.ExtKeyUsage, reference time.Time) (tls.Certificate, error) {
	chain, err := publicCertificates(chainPEM)
	if err != nil || len(chain) == 0 {
		return tls.Certificate{}, errors.New("source public chain unavailable")
	}
	roots, err := publicCertificates(rootPEM)
	if err != nil || len(roots) == 0 {
		return tls.Certificate{}, errors.New("source explicit roots unavailable")
	}
	rootPool, intermediate := x509.NewCertPool(), x509.NewCertPool()
	for _, root := range roots {
		if !root.IsCA || !root.BasicConstraintsValid || root.KeyUsage&x509.KeyUsageCertSign == 0 {
			return tls.Certificate{}, errors.New("source root is not a CA")
		}
		rootPool.AddCert(root)
	}
	var certificate tls.Certificate
	for i, item := range chain {
		certificate.Certificate = append(certificate.Certificate, bytes.Clone(item.Raw))
		if i > 0 {
			intermediate.AddCert(item)
		}
	}
	if chain[0].IsCA {
		return tls.Certificate{}, errors.New("source leaf cannot be a CA")
	}
	if _, err := chain[0].Verify(x509.VerifyOptions{Roots: rootPool, Intermediates: intermediate, DNSName: name, CurrentTime: reference, KeyUsages: []x509.ExtKeyUsage{usage}}); err != nil {
		return tls.Certificate{}, errors.New("source public chain purpose/name/time verification failed")
	}
	return certificate, validateCredential(certificate, true)
}
