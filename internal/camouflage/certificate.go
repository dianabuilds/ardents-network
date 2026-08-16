package camouflage

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"path/filepath"
)

func loadServerCertificate(config Config, certificatePath, keyPath string) (tls.Certificate, error) {
	if !regularBoundedFile(certificatePath, 64<<10, false) || !regularBoundedFile(keyPath, 64<<10, true) {
		return tls.Certificate{}, errors.New("adapter-certificate-invalid")
	}
	certificate, err := tls.LoadX509KeyPair(certificatePath, keyPath)
	if err != nil || len(certificate.Certificate) == 0 {
		return tls.Certificate{}, errors.New("adapter-certificate-invalid")
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil || leaf.VerifyHostname(config.serverName) != nil {
		return tls.Certificate{}, errors.New("adapter-certificate-invalid")
	}
	pin := certificateChainPin(certificate.Certificate)
	if subtle.ConstantTimeCompare(pin[:], config.chainPin[:]) != 1 {
		return tls.Certificate{}, errors.New("adapter-certificate-invalid")
	}
	return certificate, nil
}

func certificateChainPin(chain [][]byte) [32]byte {
	var result [32]byte
	for index, encoded := range chain {
		digest := sha256.Sum256(encoded)
		if index == 0 {
			result = digest
			continue
		}
		combined := make([]byte, 0, 64)
		combined = append(combined, result[:]...)
		combined = append(combined, digest[:]...)
		result = sha256.Sum256(combined)
	}
	return result
}

func regularBoundedFile(path string, maximum int64, ownerOnly bool) bool {
	if path == "" || !filepath.IsAbs(path) {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > maximum {
		return false
	}
	return !ownerOnly || info.Mode().Perm()&0o077 == 0
}
