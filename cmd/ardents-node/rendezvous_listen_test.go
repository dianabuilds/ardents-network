package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNodePlanProjectsOptionalRendezvousLoopbackListenOverride(t *testing.T) {
	certificatePath, keyPath, nodeID := writeRendezvousListenCredential(t)
	input := nodePlan{sourceServerPlan: sourceServerPlan{Schema: "ardents-node-plan-v1", LocalRoleStateRoot: t.TempDir()},
		NodeID: nodeID, IdentityKey: keyPath, Rendezvous: &rendezvousPlan{
			LoopbackListenOverride: "127.0.0.1:48127", HandshakeLimit: 2, WaitingLimit: 2, PairLimit: 1,
			PairByteLimit: 1024, AdmissionTimeoutMS: 1000, DrainTimeoutMS: 1000}}
	input.ServerCertificate, input.ServerKey = certificatePath, keyPath
	path := filepath.Join(t.TempDir(), "node-plan.json")
	raw, err := json.Marshal(input)
	if err != nil || os.WriteFile(path, raw, 0o600) != nil {
		t.Fatal("write Rendezvous plan")
	}
	var decoded nodePlan
	if err := decodeOperatorInput(path, 64<<10, &decoded); err != nil {
		t.Fatal(err)
	}
	configured, err := loadNodeIdentity(decoded, [32]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	if configured.Rendezvous.LoopbackListenOverride != "127.0.0.1:48127" {
		t.Fatalf("Rendezvous loopback listen override = %q", configured.Rendezvous.LoopbackListenOverride)
	}

	decoded.Rendezvous.LoopbackListenOverride = ""
	configured, err = loadNodeIdentity(decoded, [32]byte{1})
	if err != nil || configured.Rendezvous.LoopbackListenOverride != "" {
		t.Fatalf("omitted Rendezvous loopback listen override = %q / %v", configured.Rendezvous.LoopbackListenOverride, err)
	}
}

func writeRendezvousListenCredential(t *testing.T) (string, string, string) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "rendezvous-listen.test"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	certificate, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	certificatePath := filepath.Join(root, "rendezvous.pem")
	keyPath := filepath.Join(root, "rendezvous.key")
	if os.WriteFile(certificatePath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate}), 0o600) != nil ||
		os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}), 0o600) != nil {
		t.Fatal("write Rendezvous credential")
	}
	return certificatePath, keyPath, hex.EncodeToString(make([]byte, 32))
}
