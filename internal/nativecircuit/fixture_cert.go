package nativecircuit

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

type endpointFixture struct {
	rootPEM      []byte
	chainPEM     []byte
	privatePEM   []byte
	leafSHA256   string
	targetMarker []byte
	rootCert     *x509.Certificate
	rootKey      ed25519.PrivateKey
}

func generateNodeIdentity(directory, role string) (roleHop, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return roleHop{}, err
	}
	serial, err := randomSerial()
	if err != nil {
		return roleHop{}, err
	}
	template := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: "Carrier Lab " + role}, DNSNames: []string{nodeServerName},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		return roleHop{}, err
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return roleHop{}, err
	}
	if err := os.WriteFile(filepath.Join(directory, "node.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		return roleHop{}, err
	}
	if err := os.WriteFile(filepath.Join(directory, "node.key"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}), 0o600); err != nil {
		return roleHop{}, err
	}
	digest := sha256.Sum256(der)
	return roleHop{Address: role + ":37001", CertificateSHA256: hex.EncodeToString(digest[:])}, nil
}

func generateEndpointFixture() (endpointFixture, error) {
	rootPublic, rootPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return endpointFixture{}, err
	}
	rootSerial, err := randomSerial()
	if err != nil {
		return endpointFixture{}, err
	}
	markerBytes := make([]byte, 16)
	if _, err := rand.Read(markerBytes); err != nil {
		return endpointFixture{}, err
	}
	marker := "target-" + hex.EncodeToString(markerBytes)
	rootTemplate := &x509.Certificate{
		SerialNumber: rootSerial, Subject: pkix.Name{CommonName: marker}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, rootPublic, rootPrivate)
	if err != nil {
		return endpointFixture{}, err
	}
	rootCertificate, err := x509.ParseCertificate(rootDER)
	if err != nil {
		return endpointFixture{}, err
	}
	leafPublic, leafPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return endpointFixture{}, err
	}
	leafSerial, err := randomSerial()
	if err != nil {
		return endpointFixture{}, err
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: leafSerial, Subject: pkix.Name{CommonName: "synthetic-instance"}, DNSNames: []string{endpointServerName},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, rootCertificate, leafPublic, rootPrivate)
	if err != nil {
		return endpointFixture{}, err
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(leafPrivate)
	if err != nil {
		return endpointFixture{}, err
	}
	rootPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER})
	chainPEM := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}), rootPEM...)
	digest := sha256.Sum256(leafDER)
	return endpointFixture{
		rootPEM: rootPEM, chainPEM: chainPEM, privatePEM: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}),
		leafSHA256: hex.EncodeToString(digest[:]), targetMarker: []byte(marker), rootCert: rootCertificate, rootKey: rootPrivate,
	}, nil
}

func generateAlternateEndpointLeaf(root endpointFixture) ([]byte, []byte, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}
	template := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: "otherwise-valid-wrong-instance"}, DNSNames: []string{endpointServerName},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, template, root.rootCert, publicKey, root.rootKey)
	if err != nil {
		return nil, nil, err
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, err
	}
	chain := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}), root.rootPEM...)
	key := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})
	return chain, key, nil
}

func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 120)
	return rand.Int(rand.Reader, limit)
}
