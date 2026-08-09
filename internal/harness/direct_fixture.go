package harness

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

type directFixture struct {
	targetRoot        string
	activeCertificate string
	activePrivateKey  string
	activeLeafSHA256  string
	wrongCertificate  string
	wrongPrivateKey   string
	canaryHex         string
	payloadSeed       string
}

type directRoleConfigInput struct {
	SchemaVersion      string `json:"schema_version"`
	RunID              string `json:"run_id"`
	Case               string `json:"case"`
	Role               string `json:"role"`
	Address            string `json:"address"`
	CertificatePath    string `json:"certificate_path,omitempty"`
	PrivateKeyPath     string `json:"private_key_path,omitempty"`
	TargetRootPath     string `json:"target_root_path,omitempty"`
	ExpectedLeafSHA256 string `json:"expected_leaf_sha256,omitempty"`
	CanaryHex          string `json:"canary_hex,omitempty"`
	PayloadSeed        string `json:"payload_seed,omitempty"`
	PayloadSize        int    `json:"payload_size,omitempty"`
}

type directTamperConfigInput struct {
	SchemaVersion  string `json:"schema_version"`
	RunID          string `json:"run_id"`
	ListenAddress  string `json:"listen_address"`
	ServiceAddress string `json:"service_address"`
}

func prepareDirectFixture(runDir string) (directFixture, error) {
	fixtureDir := filepath.Join(runDir, "direct-fixture")
	if err := os.Mkdir(fixtureDir, 0o700); err != nil {
		return directFixture{}, err
	}
	_, targetKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return directFixture{}, err
	}
	targetSerial, err := randomSerial()
	if err != nil {
		return directFixture{}, err
	}
	targetTemplate := &x509.Certificate{
		SerialNumber: targetSerial, Subject: pkix.Name{CommonName: "Carrier Lab Target"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	targetDER, err := x509.CreateCertificate(rand.Reader, targetTemplate, targetTemplate, targetKey.Public(), targetKey)
	if err != nil {
		return directFixture{}, err
	}
	targetCertificate, err := x509.ParseCertificate(targetDER)
	if err != nil {
		return directFixture{}, err
	}
	fixture := directFixture{targetRoot: filepath.Join(fixtureDir, "target-root.pem")}
	if err := writePEMFile(fixture.targetRoot, "CERTIFICATE", targetDER); err != nil {
		return directFixture{}, err
	}
	fixture.activeCertificate, fixture.activePrivateKey, fixture.activeLeafSHA256, err = writeInstanceFixture(fixtureDir, "active-instance", targetDER, targetCertificate, targetKey)
	if err != nil {
		return directFixture{}, err
	}
	fixture.wrongCertificate, fixture.wrongPrivateKey, _, err = writeInstanceFixture(fixtureDir, "wrong-instance", targetDER, targetCertificate, targetKey)
	if err != nil {
		return directFixture{}, err
	}
	canary := make([]byte, 32)
	seed := make([]byte, 32)
	if _, err := rand.Read(canary); err != nil {
		return directFixture{}, err
	}
	if _, err := rand.Read(seed); err != nil {
		return directFixture{}, err
	}
	fixture.canaryHex = hex.EncodeToString(canary)
	fixture.payloadSeed = hex.EncodeToString(seed)
	return fixture, nil
}

func writeInstanceFixture(directory, name string, targetDER []byte, targetCertificate *x509.Certificate, targetKey ed25519.PrivateKey) (certificatePath, keyPath, leafSHA256 string, err error) {
	_, instanceKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", "", err
	}
	leafSerial, err := randomSerial()
	if err != nil {
		return "", "", "", err
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: leafSerial, Subject: pkix.Name{CommonName: name}, DNSNames: []string{"carrier.invalid"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, targetCertificate, instanceKey.Public(), targetKey)
	if err != nil {
		return "", "", "", err
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(instanceKey)
	if err != nil {
		return "", "", "", err
	}
	certificatePath = filepath.Join(directory, name+"-chain.pem")
	keyPath = filepath.Join(directory, name+"-key.pem")
	chain := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: targetDER})...)
	if err := os.WriteFile(certificatePath, chain, 0o600); err != nil {
		return "", "", "", err
	}
	if err := writePEMFile(keyPath, "PRIVATE KEY", privateDER); err != nil {
		return "", "", "", err
	}
	digest := sha256.Sum256(leafDER)
	return certificatePath, keyPath, hex.EncodeToString(digest[:]), nil
}

func writePEMFile(path, blockType string, data []byte) error {
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: data}), 0o600)
}

func randomSerial() (*big.Int, error) {
	serial := make([]byte, 16)
	if _, err := rand.Read(serial); err != nil {
		return nil, err
	}
	serial[0] &= 0x7f
	value := new(big.Int).SetBytes(serial)
	if value.Sign() == 0 {
		value.SetInt64(1)
	}
	return value, nil
}
