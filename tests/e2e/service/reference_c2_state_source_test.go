package service_test

import (
	"bufio"
	"bytes"
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
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

type referenceC2SourceCredential struct {
	certificate, privateKey, root, rootPath string
	leafDigest                              [32]byte
	authority                               ed25519.PrivateKey
}

type referenceC2SourceEndpoint struct {
	address, serverName, root string
	leafDigest                [32]byte
}

func referenceC2StartStateSources(t *testing.T, ctx context.Context, nodeBinary string, fixture referenceC2StateFixture,
	root string) ([2]referenceC2SourceEndpoint, referenceC2SourceCredential) {
	t.Helper()
	clientAuthority := referenceC2SourceAuthority(t, "reference-c2-source-client-root", 41)
	client := referenceC2SourceLeaf(t, clientAuthority, "reference-c2-source-client", 42, false)
	public := fixture.authority.Public().(ed25519.PublicKey)
	var endpoints [2]referenceC2SourceEndpoint
	for index := range endpoints {
		serverAuthority := referenceC2SourceAuthority(t, "reference-c2-source-server-root-"+string(rune('a'+index)), int64(43+index))
		server := referenceC2SourceLeaf(t, serverAuthority, "reference-c2-source-server-"+string(rune('a'+index)), int64(45+index), true)
		address := referenceC2Address(t)
		stateRoot := filepath.Join(root, "source-"+string(rune('a'+index))+"-state")
		referenceC2AcceptState(t, fixture, stateRoot, "rendezvous")
		plan := map[string]any{"schema": "ardents-source-server-v1", "state_root": stateRoot,
			"local_role_state_root": stateRoot + "-roles", "network_id": referenceC2Hex(fixture.network),
			"authority_public": []string{hex.EncodeToString(public)}, "threshold": 1, "at": fixture.now.Format(time.RFC3339),
			"listen": address, "server_certificate": server.certificate, "server_key": server.privateKey,
			"client_root": clientAuthority.rootPath, "client_key_digests": []string{hex.EncodeToString(client.leafDigest[:])},
			"materialization_index": 0, "native_rendezvous_profile": true}
		path := filepath.Join(root, "source-"+string(rune('a'+index))+".json")
		raw, err := json.Marshal(plan)
		if err != nil || os.WriteFile(path, raw, 0o600) != nil {
			t.Fatal("write reference C2 State source plan")
		}
		referenceC2StartStateSource(t, ctx, nodeBinary, path)
		endpoints[index] = referenceC2SourceEndpoint{address: address, serverName: "reference-c2-source-server-" + string(rune('a'+index)),
			root: serverAuthority.root, leafDigest: server.leafDigest}
	}
	return endpoints, client
}

func referenceC2StartStateSource(t *testing.T, ctx context.Context, binary, plan string) {
	t.Helper()
	command := exec.CommandContext(ctx, binary, "source", "--config", plan)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if command.ProcessState == nil {
			_ = command.Process.Kill()
		}
		_ = command.Wait()
	})
	ready := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		ready <- scanner.Scan() && bytes.Contains(scanner.Bytes(), []byte(`"kind":"source-ready"`))
	}()
	select {
	case ok := <-ready:
		if !ok {
			t.Fatalf("reference C2 State source did not become ready: %s", stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("reference C2 State source readiness timed out")
	}
}

func referenceC2SourceAuthority(t *testing.T, name string, serial int64) referenceC2SourceCredential {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	root := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return referenceC2SourceCredential{root: string(root), rootPath: writeReferenceC2PEM(t, name+".pem", "CERTIFICATE", der), authority: private}
}

func referenceC2SourceLeaf(t *testing.T, authority referenceC2SourceCredential, name string, serial int64, server bool) referenceC2SourceCredential {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode([]byte(authority.root))
	if block == nil || len(authority.authority) != ed25519.PrivateKeySize {
		t.Fatal("reference C2 source authority material is unavailable")
	}
	root, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	usage := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	if server {
		usage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: usage, DNSNames: []string{name}}
	der, err := x509.CreateCertificate(rand.Reader, template, root, public, authority.authority)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	credential := referenceC2SourceCredential{certificate: writeReferenceC2PEM(t, name+".pem", "CERTIFICATE", der),
		privateKey: writeReferenceC2PEM(t, name+".key", "PRIVATE KEY", privateDER)}
	credential.leafDigest = sha256.Sum256(append([]byte("ardents-h3-source-transport-key-v1\x00"), public...))
	return credential
}

func writeReferenceC2PEM(t *testing.T, name, kind string, der []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
