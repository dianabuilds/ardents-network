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

type referenceC2StateSources struct {
	endpoints [2]referenceC2SourceEndpoint
	client    referenceC2SourceCredential
	servers   [2]*referenceC2StateSource
}

type referenceC2StateSource struct {
	binary, plan, stateRoot string
	context                 context.Context
	command                 *exec.Cmd
	done                    chan struct{}
	stderr                  *bytes.Buffer
}

func referenceC2StartStateSources(t *testing.T, ctx context.Context, nodeBinary string, fixture referenceC2StateFixture,
	root string) referenceC2StateSources {
	t.Helper()
	clientAuthority := referenceC2SourceAuthority(t, "reference-c2-source-client-root", 41)
	client := referenceC2SourceLeaf(t, clientAuthority, "reference-c2-source-client", 42, false)
	public := fixture.authority.Public().(ed25519.PublicKey)
	result := referenceC2StateSources{client: client}
	for index := range result.endpoints {
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
		source := &referenceC2StateSource{binary: nodeBinary, plan: path, stateRoot: stateRoot, context: ctx}
		source.start(t)
		t.Cleanup(func() { source.stop(t) })
		result.servers[index] = source
		result.endpoints[index] = referenceC2SourceEndpoint{address: address, serverName: "reference-c2-source-server-" + string(rune('a'+index)),
			root: serverAuthority.root, leafDigest: server.leafDigest}
	}
	return result
}

func (source *referenceC2StateSource) replaceState(t *testing.T, fixture referenceC2StateFixture) {
	t.Helper()
	source.stop(t)
	referenceC2AcceptState(t, fixture, source.stateRoot, "rendezvous")
	source.start(t)
}

func (source *referenceC2StateSource) start(t *testing.T) {
	t.Helper()
	command := exec.CommandContext(source.context, source.binary, "source", "--config", source.plan)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	source.stderr = new(bytes.Buffer)
	command.Stderr = source.stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	source.command, source.done = command, make(chan struct{})
	go func() {
		_ = command.Wait()
		close(source.done)
	}()
	ready := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		ready <- scanner.Scan() && bytes.Contains(scanner.Bytes(), []byte(`"kind":"source-ready"`))
	}()
	select {
	case ok := <-ready:
		if !ok {
			t.Fatalf("reference C2 State source did not become ready: %s", source.stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("reference C2 State source readiness timed out")
	}
}

func (source *referenceC2StateSource) stop(t *testing.T) {
	t.Helper()
	if source.command == nil || source.done == nil {
		return
	}
	running := true
	select {
	case <-source.done:
		running = false
	default:
	}
	if running && source.command.Process != nil {
		_ = source.command.Process.Kill()
	}
	select {
	case <-source.done:
	case <-time.After(2 * time.Second):
		t.Fatalf("reference C2 State source did not stop: %s", source.stderr.String())
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
