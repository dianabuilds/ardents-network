package waku

import (
	"ardents/internal/network"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"time"

	persistence "ardents/internal/storage"
)

func ValidateConfig(cfg network.Config, now time.Time) error {
	if err := validateLimits(cfg.Limits); err != nil {
		return err
	}
	if err := validateReachabilityConfig(cfg); err != nil {
		return err
	}
	if err := validateDiscoveryConfig(cfg); err != nil {
		return err
	}
	profile := network.NormalizeProfile(cfg.Profile)
	if profile != network.ProfileTCPWSS {
		if hasWSSConfig(cfg) {
			return fmt.Errorf("transport profile %q does not accept secure websocket settings", profile)
		}
		return nil
	}
	if strings.TrimSpace(cfg.WSSCertPath) == "" || strings.TrimSpace(cfg.WSSKeyPath) == "" {
		return fmt.Errorf("transport profile %q requires secure websocket certificate and key paths", profile)
	}
	if cfg.WSSPort < 1 || cfg.WSSPort > 65535 {
		return fmt.Errorf("secure websocket port must be between 1 and 65535")
	}
	host := strings.TrimSpace(cfg.WSSAdvertiseAddress)
	if host == "" {
		return fmt.Errorf("secure websocket advertised address is required")
	}
	return validateWSSMaterial(cfg, host, now)
}

func validateWSSMaterial(cfg network.Config, host string, now time.Time) error {
	certRaw, err := readCertificateFile(cfg.WSSCertPath)
	if err != nil {
		return err
	}
	keyRaw, found, err := persistence.ReadPrivateFile(strings.TrimSpace(cfg.WSSKeyPath))
	if err != nil {
		return fmt.Errorf("secure websocket private key is unreadable or insecure")
	}
	if !found {
		return fmt.Errorf("secure websocket private key file does not exist")
	}
	pair, err := tls.X509KeyPair(certRaw, keyRaw)
	if err != nil {
		return fmt.Errorf("secure websocket certificate and private key are invalid or mismatched")
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return fmt.Errorf("secure websocket leaf certificate is invalid")
	}
	if err := validateLeafCertificate(leaf, host, now); err != nil {
		return err
	}
	return validateCertificateChain(pair.Certificate, leaf, cfg.WSSCAPath, host, now)
}

func hasWSSConfig(cfg network.Config) bool {
	return cfg.WSSPort != 0 || strings.TrimSpace(cfg.WSSCertPath) != "" ||
		strings.TrimSpace(cfg.WSSKeyPath) != "" || strings.TrimSpace(cfg.WSSCAPath) != "" ||
		strings.TrimSpace(cfg.WSSAdvertiseAddress) != ""
}

func readCertificateFile(path string) ([]byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("secure websocket certificate path is required")
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("secure websocket certificate file does not exist")
	}
	if err != nil {
		return nil, fmt.Errorf("secure websocket certificate cannot be inspected")
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("secure websocket certificate must be a regular file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("secure websocket certificate is unreadable")
	}
	return raw, nil
}

func validateLeafCertificate(leaf *x509.Certificate, host string, now time.Time) error {
	if now.Before(leaf.NotBefore) {
		return fmt.Errorf("secure websocket certificate is not valid yet")
	}
	if now.After(leaf.NotAfter) {
		return fmt.Errorf("secure websocket certificate is expired")
	}
	if leaf.IsCA {
		return fmt.Errorf("secure websocket certificate must be a server leaf certificate")
	}
	if bytes.Equal(leaf.RawIssuer, leaf.RawSubject) &&
		leaf.CheckSignature(leaf.SignatureAlgorithm, leaf.RawTBSCertificate, leaf.Signature) == nil {
		return fmt.Errorf("secure websocket certificate must not be self-signed")
	}
	if err := leaf.VerifyHostname(host); err != nil {
		return fmt.Errorf("secure websocket certificate does not cover advertised address")
	}
	if !allowsServerAuth(leaf.ExtKeyUsage) {
		return fmt.Errorf("secure websocket certificate does not allow server authentication")
	}
	return nil
}

func allowsServerAuth(usages []x509.ExtKeyUsage) bool {
	if len(usages) == 0 {
		return true
	}
	for _, usage := range usages {
		if usage == x509.ExtKeyUsageServerAuth || usage == x509.ExtKeyUsageAny {
			return true
		}
	}
	return false
}

func validateCertificateChain(chain [][]byte, leaf *x509.Certificate, caPath, host string, now time.Time) error {
	roots, err := trustedRoots(strings.TrimSpace(caPath))
	if err != nil {
		return err
	}
	intermediates := x509.NewCertPool()
	for _, raw := range chain[1:] {
		certificate, err := x509.ParseCertificate(raw)
		if err != nil {
			return fmt.Errorf("secure websocket certificate chain is invalid")
		}
		intermediates.AddCert(certificate)
	}
	_, err = leaf.Verify(x509.VerifyOptions{
		Roots: roots, Intermediates: intermediates, DNSName: host, CurrentTime: now,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	if err != nil {
		return fmt.Errorf("secure websocket certificate chain is not trusted")
	}
	return nil
}

func trustedRoots(caPath string) (*x509.CertPool, error) {
	if caPath == "" {
		roots, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("system certificate roots are unavailable")
		}
		return roots, nil
	}
	raw, err := readCABundle(caPath)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(raw) {
		return nil, fmt.Errorf("secure websocket CA bundle contains no certificates")
	}
	return roots, nil
}

func readCABundle(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("secure websocket CA bundle does not exist")
	}
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("secure websocket CA bundle must be a readable regular file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("secure websocket CA bundle is unreadable")
	}
	return raw, nil
}
