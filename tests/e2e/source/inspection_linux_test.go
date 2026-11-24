package source_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/source/servingdeclaration"
)

// A command/transport control with actual generated TLS, original files and
// physical serving. It supplies no State acceptance, installed invocation,
// family independence or bootstrap evidence.
func TestSourceInspectionUsesOriginalCredentialAndOneContact(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "ardents")
	build := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-w", "-o", binary, "./cmd/ardents-next")
	build.Dir = filepath.Join("..", "..", "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v: %s", err, output)
	}

	now := time.Now()
	caPublic, caPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(caPrivate)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Source inspection control"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, caPublic, caPrivate)
	if err != nil {
		t.Fatal(err)
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	issue := func(serial int64, usage x509.ExtKeyUsage) (tls.Certificate, []byte, []byte, [32]byte) {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { clear(private) })
		leaf := &x509.Certificate{SerialNumber: big.NewInt(serial), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
			KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}, DNSNames: []string{"source.test"}}
		der, err := x509.CreateCertificate(rand.Reader, leaf, ca, public, caPrivate)
		if err != nil {
			t.Fatal(err)
		}
		keyDER, err := x509.MarshalPKCS8PrivateKey(private)
		if err != nil {
			t.Fatal(err)
		}
		defer clear(keyDER)
		certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		key := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
		t.Cleanup(func() { clear(key) })
		pin := sha256.Sum256(append([]byte("ardents-h3-source-transport-key-v1\x00"), public...))
		return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: private}, certificate, key, pin
	}
	serverCredential, _, _, serverPin := issue(2, x509.ExtKeyUsageServerAuth)
	_, clientPEM, clientKeyPEM, clientPin := issue(3, x509.ExtKeyUsageClientAuth)
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(authorityPrivate)
	var authority [32]byte
	copy(authority[:], authorityPublic)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	serving, _, err := source.New(source.Config{ServeAddress: address, ServeCertificate: serverCredential, ServeClientRootPEM: caPEM,
		ServeClientKeyDigests: [][32]byte{clientPin}, VerificationClock: time.Now}, map[[32]byte]ed25519.PublicKey{sha256.Sum256(authorityPublic): authorityPublic})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	ready, joined := make(chan error, 1), make(chan error, 1)
	var contacts atomic.Int32
	go func() {
		joined <- serving.Serve(ctx, ready, nil, nil, func(context.Context, source.Message) source.Message {
			contacts.Add(1)
			return source.Message{Status: "not-found"}
		})
	}()
	defer func() {
		cancel()
		if err := <-joined; err != context.Canceled {
			t.Error("serving join", err)
		}
	}()
	if err := <-ready; err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	paths := []string{filepath.Join(directory, "serve.json"), filepath.Join(directory, "client.pem"), filepath.Join(directory, "client-key.pem"), filepath.Join(directory, "server-ca.pem")}
	plan := servingdeclaration.ServingDeclaration{Network: &servingdeclaration.ServingNetwork{Root: "/unopened-state", NetworkID: [32]byte{11}, Authorities: [][32]byte{authority}, Threshold: 1,
		ProfileAuthority: authority, ClockObservationFile: "/unopened-clock"}, ProcessProfile: "ardents-source-process-v1", ParticipationRoot: "/unopened-participation",
		Address: address, Certificate: "/unopened-source-certificate", PrivateKey: "/unopened-source-private-key", ClientRoot: "/unopened-client-ca", ClientPins: [][32]byte{clientPin}}
	planRaw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	for i, raw := range [][]byte{planRaw, clientPEM, clientKeyPEM, caPEM} {
		mode := os.FileMode(0644)
		if i == 2 {
			mode = 0600
		}
		if err := os.WriteFile(paths[i], raw, mode); err != nil {
			t.Fatal(err)
		}
	}
	id := [32]byte{8}
	args := append([]string{"source", "inspect-latest"}, paths...)
	args = append(args, "source.test", hex.EncodeToString(serverPin[:]), hex.EncodeToString(id[:]), "one-effective-controller", "one-source", "0")
	command := exec.CommandContext(t.Context(), binary, args...)
	var diagnostic bytes.Buffer
	command.Stderr = &diagnostic
	output, err := command.Output()
	if err != nil {
		t.Fatalf("inspect Source: %v: %s", err, &diagnostic)
	}
	var report struct {
		Operation string `json:"operation"`
		Status    string `json:"status"`
	}
	if json.Unmarshal(output, &report) != nil || report.Operation != "source.inspect-latest" || report.Status != "not-found" || contacts.Load() != 1 {
		t.Fatal("actual command result", string(output), contacts.Load())
	}
	if err := os.Chmod(paths[2], 0644); err != nil {
		t.Fatal(err)
	}
	command = exec.CommandContext(t.Context(), binary, args...)
	diagnostic.Reset()
	command.Stderr = &diagnostic
	output, err = command.Output()
	if command.ProcessState == nil || command.ProcessState.ExitCode() != 1 || len(output) != 0 || contacts.Load() != 1 {
		t.Fatalf("expected credential refusal without stdout or another contact: %v, stdout %q, stderr %q, contacts %d", err, output, &diagnostic, contacts.Load())
	}
}
