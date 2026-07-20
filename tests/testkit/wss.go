package testkit

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func WriteSelfSignedWSSCert(t *testing.T) (string, string) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: "127.0.0.1",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	keyBytes := x509.MarshalPKCS1PrivateKey(priv)
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certPath, keyPath
}

func WriteWSSCert(t *testing.T) (string, string) {
	t.Helper()
	now := time.Now()
	return writeSignedWSSCert(t, now.Add(-time.Hour), now.Add(24*time.Hour), "127.0.0.1")
}

func WriteExpiredWSSCert(t *testing.T) (string, string) {
	t.Helper()
	now := time.Now()
	return writeSignedWSSCert(t, now.Add(-48*time.Hour), now.Add(-24*time.Hour), "127.0.0.1")
}

func WriteWSSCertForHost(t *testing.T, host string) (string, string) {
	t.Helper()
	now := time.Now()
	return writeSignedWSSCert(t, now.Add(-time.Hour), now.Add(24*time.Hour), host)
}

func RotateWSSCert(t *testing.T, certPath, keyPath, host string) {
	t.Helper()
	newCertPath, newKeyPath := WriteWSSCertForHost(t, host)
	copyPrivateTestFile(t, newCertPath, certPath)
	copyPrivateTestFile(t, newKeyPath, keyPath)
	copyPrivateTestFile(t, WSSCAPath(newCertPath), WSSCAPath(certPath))
}

func WSSCAPath(certPath string) string {
	return filepath.Join(filepath.Dir(certPath), "ca.pem")
}

func writeSignedWSSCert(t *testing.T, notBefore, notAfter time.Time, host string) (string, string) {
	t.Helper()

	caKey := generateRSAKey(t)
	now := time.Now()
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(now.UnixNano()),
		Subject:               pkix.Name{CommonName: "Ardents test CA"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(48 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA certificate: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse CA certificate: %v", err)
	}

	serverKey := generateRSAKey(t)
	serverTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(now.UnixNano() + 1),
		Subject:               pkix.Name{CommonName: host},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	if ip := net.ParseIP(host); ip != nil {
		serverTemplate.IPAddresses = []net.IP{ip}
	} else {
		serverTemplate.DNSNames = []string{host}
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create server certificate: %v", err)
	}
	certPath, keyPath := writeWSSMaterial(t, serverDER, serverKey)
	caRaw := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	if err := os.WriteFile(WSSCAPath(certPath), caRaw, 0o600); err != nil {
		t.Fatalf("write CA certificate: %v", err)
	}
	return certPath, keyPath
}

func generateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	return key
}

func writeWSSMaterial(t *testing.T, der []byte, key *rsa.PrivateKey) (string, string) {
	t.Helper()
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certPath, keyPath
}

func copyPrivateTestFile(t *testing.T, source, target string) {
	t.Helper()
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read rotated WSS material: %v", err)
	}
	if err := os.WriteFile(target, raw, 0o600); err != nil {
		t.Fatalf("write rotated WSS material: %v", err)
	}
}
