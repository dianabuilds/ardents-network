package directcontrol

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDirectRoleRejectsKnowledgeOutsideItsFixedConfig(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	configPath := filepath.Join(root, "role.json")
	evidenceDir := filepath.Join(root, "evidence")
	if err := os.Mkdir(evidenceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	config := `{"schema_version":"carrier-lab-direct-role/v1","run_id":"20260809T140000Z-direct","case":"positive","role":"user","address":"127.0.0.1:1","target_name":"forbidden"}`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RunRole(context.Background(), configPath, evidenceDir); err == nil {
		t.Fatal("Direct TLS role accepted undeclared target naming knowledge")
	}
}

func TestDirectTLSControlAuthenticatesExactInstanceAndCanary(t *testing.T) {
	root := t.TempDir()
	address := reserveDirectAddress(t)
	targetRoot, certificate, privateKey, leafHash := writeDirectFixture(t, root, "active-instance")
	serviceEvidence := filepath.Join(root, "service-evidence")
	userEvidence := filepath.Join(root, "user-evidence")
	for _, directory := range []string{serviceEvidence, userEvidence} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	runID := "20260809T140100Z-direct"
	serviceConfig := filepath.Join(root, "service.json")
	userConfig := filepath.Join(root, "user.json")
	writeDirectConfig(t, serviceConfig, directRoleConfig{
		SchemaVersion: directRoleSchema, RunID: runID, Case: "positive", Role: "service", Address: address,
		CertificatePath: certificate, PrivateKeyPath: privateKey,
	})
	writeDirectConfig(t, userConfig, directRoleConfig{
		SchemaVersion: directRoleSchema, RunID: runID, Case: "positive", Role: "user", Address: address,
		TargetRootPath: targetRoot, ExpectedLeafSHA256: leafHash,
		CanaryHex: hex.EncodeToString([]byte("0123456789abcdef0123456789abcdef")), PayloadSeed: "direct-positive-seed", PayloadSize: 4096,
	})

	serviceDone := make(chan error, 1)
	go func() { serviceDone <- RunRole(context.Background(), serviceConfig, serviceEvidence) }()
	waitForDirectReady(t, filepath.Join(serviceEvidence, "ready.json"), serviceDone)
	if err := RunRole(context.Background(), userConfig, userEvidence); err != nil {
		t.Fatal(err)
	}
	if err := <-serviceDone; err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(userEvidence, "result.json"), filepath.Join(serviceEvidence, "result.json")} {
		var result map[string]any
		loadDirectJSON(t, path, &result)
		if result["status"] != "passed" || result["tls_version"] != "TLS1.3" || result["curve"] != "X25519" || result["session_resumed"] != false {
			t.Fatalf("unexpected Direct TLS evidence in %s: %#v", path, result)
		}
		if result["application_bytes_verified"] != true {
			t.Fatalf("unverified Application bytes in %s: %#v", path, result)
		}
		if result["elapsed_milliseconds"].(float64) < 0 || result["heap_alloc_bytes"].(float64) <= 0 || result["goroutines"].(float64) <= 0 {
			t.Fatalf("missing bounded resource evidence in %s: %#v", path, result)
		}
	}
}

func TestDirectTLSControlRejectsWrongInstanceBeforeApplicationBytes(t *testing.T) {
	root := t.TempDir()
	address := reserveDirectAddress(t)
	targetRoot, _, _, activeLeafHash, wrongCertificate, wrongKey := writeDirectFixturePair(t, root)
	serviceEvidence := filepath.Join(root, "service-evidence")
	userEvidence := filepath.Join(root, "user-evidence")
	for _, directory := range []string{serviceEvidence, userEvidence} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	runID := "20260809T140200Z-direct-wrong-instance"
	serviceConfig := filepath.Join(root, "service.json")
	userConfig := filepath.Join(root, "user.json")
	writeDirectConfig(t, serviceConfig, directRoleConfig{
		SchemaVersion: directRoleSchema, RunID: runID, Case: "wrong-instance", Role: "service", Address: address,
		CertificatePath: wrongCertificate, PrivateKeyPath: wrongKey,
	})
	writeDirectConfig(t, userConfig, directRoleConfig{
		SchemaVersion: directRoleSchema, RunID: runID, Case: "wrong-instance", Role: "user", Address: address,
		TargetRootPath: targetRoot, ExpectedLeafSHA256: activeLeafHash,
		CanaryHex: hex.EncodeToString([]byte("0123456789abcdef0123456789abcdef")), PayloadSeed: "forbidden-payload", PayloadSize: 4096,
	})

	serviceDone := make(chan error, 1)
	go func() { serviceDone <- RunRole(context.Background(), serviceConfig, serviceEvidence) }()
	waitForDirectReady(t, filepath.Join(serviceEvidence, "ready.json"), serviceDone)
	if err := RunRole(context.Background(), userConfig, userEvidence); err == nil {
		t.Fatal("wrong Instance leaf was accepted")
	}
	if err := <-serviceDone; err == nil {
		t.Fatal("service did not observe the rejected handshake")
	}
	var result map[string]any
	loadDirectJSON(t, filepath.Join(userEvidence, "result.json"), &result)
	if result["status"] != "failed" || result["terminal_result"] != "explicit_failure" || result["application_bytes_verified"] != false {
		t.Fatalf("wrong Instance did not fail closed: %#v", result)
	}
}

func TestDirectTLSControlRejectsModifiedProtectedRecord(t *testing.T) {
	root := t.TempDir()
	serviceAddress := reserveDirectAddress(t)
	proxyAddress := reserveDirectAddress(t)
	targetRoot, certificate, privateKey, leafHash := writeDirectFixture(t, root, "tamper-instance")
	serviceEvidence := filepath.Join(root, "service-evidence")
	userEvidence := filepath.Join(root, "user-evidence")
	proxyEvidence := filepath.Join(root, "proxy-evidence")
	for _, directory := range []string{serviceEvidence, userEvidence, proxyEvidence} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	runID := "20260809T140300Z-direct-tamper"
	serviceConfig := filepath.Join(root, "service.json")
	userConfig := filepath.Join(root, "user.json")
	proxyConfig := filepath.Join(root, "proxy.json")
	writeDirectConfig(t, serviceConfig, directRoleConfig{
		SchemaVersion: directRoleSchema, RunID: runID, Case: "modified-record", Role: "service", Address: serviceAddress,
		CertificatePath: certificate, PrivateKeyPath: privateKey,
	})
	writeDirectConfig(t, userConfig, directRoleConfig{
		SchemaVersion: directRoleSchema, RunID: runID, Case: "modified-record", Role: "user", Address: proxyAddress,
		TargetRootPath: targetRoot, ExpectedLeafSHA256: leafHash,
		CanaryHex: hex.EncodeToString([]byte("0123456789abcdef0123456789abcdef")), PayloadSeed: "direct-tamper-seed", PayloadSize: 4096,
	})
	writeDirectTamperConfig(t, proxyConfig, directTamperConfig{
		SchemaVersion: directTamperSchema, RunID: runID, ListenAddress: proxyAddress, ServiceAddress: serviceAddress,
	})

	serviceDone := make(chan error, 1)
	proxyDone := make(chan error, 1)
	go func() { serviceDone <- RunRole(context.Background(), serviceConfig, serviceEvidence) }()
	waitForDirectReady(t, filepath.Join(serviceEvidence, "ready.json"), serviceDone)
	go func() { proxyDone <- RunTamper(context.Background(), proxyConfig, proxyEvidence) }()
	waitForDirectReady(t, filepath.Join(proxyEvidence, "ready.json"), proxyDone)
	if err := RunRole(context.Background(), userConfig, userEvidence); err == nil {
		t.Fatal("modified protected record was accepted")
	}
	if err := <-proxyDone; err != nil {
		t.Fatal(err)
	}
	<-serviceDone
	var userResult map[string]any
	loadDirectJSON(t, filepath.Join(userEvidence, "result.json"), &userResult)
	if userResult["status"] != "failed" || userResult["application_bytes_verified"] != false {
		t.Fatalf("modified bytes reached the Application side: %#v", userResult)
	}
	var proxyResult map[string]any
	loadDirectJSON(t, filepath.Join(proxyEvidence, "result.json"), &proxyResult)
	if proxyResult["status"] != "passed" || proxyResult["protected_record_modified"] != true {
		t.Fatalf("fault proxy did not prove the protected-record modification: %#v", proxyResult)
	}
}

func reserveDirectAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func writeDirectFixture(t *testing.T, root, instance string) (rootPath, certificatePath, keyPath, leafHash string) {
	t.Helper()
	_, targetKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	targetTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Carrier Lab Target"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	targetDER, err := x509.CreateCertificate(rand.Reader, targetTemplate, targetTemplate, targetKey.Public(), targetKey)
	if err != nil {
		t.Fatal(err)
	}
	targetCertificate, err := x509.ParseCertificate(targetDER)
	if err != nil {
		t.Fatal(err)
	}
	_, instanceKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: instance}, DNSNames: []string{"carrier.invalid"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, targetCertificate, instanceKey.Public(), targetKey)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(instanceKey)
	if err != nil {
		t.Fatal(err)
	}
	rootPath = filepath.Join(root, instance+"-root.pem")
	certificatePath = filepath.Join(root, instance+"-chain.pem")
	keyPath = filepath.Join(root, instance+"-key.pem")
	writeDirectPEM(t, rootPath, "CERTIFICATE", targetDER)
	chain := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: targetDER})...)
	if err := os.WriteFile(certificatePath, chain, 0o600); err != nil {
		t.Fatal(err)
	}
	writeDirectPEM(t, keyPath, "PRIVATE KEY", privateDER)
	digest := sha256.Sum256(leafDER)
	return rootPath, certificatePath, keyPath, hex.EncodeToString(digest[:])
}

func writeDirectFixturePair(t *testing.T, root string) (rootPath, activeCertificate, activeKey, activeLeafHash, wrongCertificate, wrongKey string) {
	t.Helper()
	_, targetKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	targetTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(10), Subject: pkix.Name{CommonName: "Carrier Lab Target"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	targetDER, err := x509.CreateCertificate(rand.Reader, targetTemplate, targetTemplate, targetKey.Public(), targetKey)
	if err != nil {
		t.Fatal(err)
	}
	targetCertificate, err := x509.ParseCertificate(targetDER)
	if err != nil {
		t.Fatal(err)
	}
	rootPath = filepath.Join(root, "paired-target-root.pem")
	writeDirectPEM(t, rootPath, "CERTIFICATE", targetDER)
	activeCertificate, activeKey, activeLeafHash = writeDirectLeaf(t, root, "active-instance", 11, targetDER, targetCertificate, targetKey)
	wrongCertificate, wrongKey, _ = writeDirectLeaf(t, root, "wrong-instance", 12, targetDER, targetCertificate, targetKey)
	return rootPath, activeCertificate, activeKey, activeLeafHash, wrongCertificate, wrongKey
}

func writeDirectLeaf(t *testing.T, root, instance string, serial int64, targetDER []byte, targetCertificate *x509.Certificate, targetKey ed25519.PrivateKey) (certificatePath, keyPath, leafHash string) {
	t.Helper()
	_, instanceKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: instance}, DNSNames: []string{"carrier.invalid"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, targetCertificate, instanceKey.Public(), targetKey)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(instanceKey)
	if err != nil {
		t.Fatal(err)
	}
	certificatePath = filepath.Join(root, instance+"-paired-chain.pem")
	keyPath = filepath.Join(root, instance+"-paired-key.pem")
	chain := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: targetDER})...)
	if err := os.WriteFile(certificatePath, chain, 0o600); err != nil {
		t.Fatal(err)
	}
	writeDirectPEM(t, keyPath, "PRIVATE KEY", privateDER)
	digest := sha256.Sum256(leafDER)
	return certificatePath, keyPath, hex.EncodeToString(digest[:])
}

func writeDirectPEM(t *testing.T, path, blockType string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: data}), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeDirectConfig(t *testing.T, path string, config directRoleConfig) {
	t.Helper()
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeDirectTamperConfig(t *testing.T, path string, config directTamperConfig) {
	t.Helper()
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func waitForDirectReady(t *testing.T, path string, done <-chan error) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		select {
		case err := <-done:
			t.Fatalf("Direct TLS role exited before ready: %v", err)
		case <-time.After(10 * time.Millisecond):
		}
	}
	t.Fatal("Direct TLS service did not become ready")
}

func loadDirectJSON(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
}
