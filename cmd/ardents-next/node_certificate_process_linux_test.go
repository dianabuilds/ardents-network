//go:build linux && node_process_native

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// Only State/Admission/cohort authorities are fixtures. Every selected Node key,
// canonical prepared and signed initial Record, and certificate comes from the
// genuine command. Actual original paths are passed unchanged to Node runtime.
func provisionNativeNodeCertificates(t *testing.T, f *networkAdmissionFixture, certificates map[[32]byte]tls.Certificate, ids ...[32]byte) map[[32]byte][2]string {
	t.Helper()
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	p, err := openPreparationParent(parent)
	if err != nil {
		t.Fatal("invalid native environment", err)
	}
	if err := signingAncestors(p); err != nil {
		_ = p.close()
		t.Fatal("invalid root-controlled ancestors", err)
	}
	if err := p.close(); err != nil {
		t.Fatal(err)
	}
	command := nodeNativeCommand(t)
	run := func(args ...string) {
		t.Helper()
		if raw, err := exec.CommandContext(t.Context(), command, append([]string{"network"}, args...)...).CombinedOutput(); err != nil {
			t.Fatal("genuine Node provisioning", err, string(raw))
		}
	}
	write := func(name string, value any) string {
		t.Helper()
		path := filepath.Join(parent, name)
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	keyPlan := write("key-plan", map[string]any{"schema": "ardents-initial-keys-plan-v1", "operation": "prepare-initial-keys", "node_count": len(ids)})
	keyRoot := filepath.Join(parent, "keys")
	run("prepare-initial-keys", keyPlan, keyRoot)
	paths := make(map[[32]byte][2]string)
	for i, id := range ids {
		keyPath := filepath.Join(keyRoot, fmt.Sprintf("node-%02d-key.pem", i))
		raw, err := os.ReadFile(keyPath)
		if err != nil {
			t.Fatal(err)
		}
		key, err := parseNetworkSigningKey(raw)
		clear(raw)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { clear(key) })
		found := false
		for j := range f.spec.Nodes {
			if f.spec.Nodes[j].NodeID == id {
				f.spec.Nodes[j].PrivateKey = key
				f.spec.Nodes[j].Generation = 1
				found = true
			}
		}
		if !found {
			t.Fatal("unknown target Node")
		}
		paths[id] = [2]string{"", keyPath}
	}
	// Independent fixture supplies only isolated State/profile/Admission and
	// fixed family assignment. Its Record bytes must equal genuine producer bytes.
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	f.spec.Nodes = bundle.Nodes
	for i, n := range bundle.Nodes {
		pair, selected := paths[n.NodeID]
		if !selected {
			continue
		}
		now := time.Now().UTC().Truncate(time.Second)
		public := map[string]any{"schema": "ardents-initial-node-record-plan-v1", "network_id": hex.EncodeToString(f.spec.NetworkID[:]), "node_id": hex.EncodeToString(n.NodeID[:]), "valid_from": n.ValidFrom.Format(time.RFC3339), "valid_until": n.ValidUntil.Format(time.RFC3339), "family": n.Family, "endpoint": n.Endpoint, "carrier": n.Carrier, "capacity": n.Capacity, "public_key": hex.EncodeToString(n.PrivateKey[32:])}
		label := fmt.Sprintf("%02d", i)
		plan := write("record-plan-"+label, public)
		prepared := filepath.Join(parent, "prepared-"+label)
		run("prepare-initial-node-record", plan, prepared)
		messagePath := filepath.Join(prepared, "node-record.signing-input")
		message, err := os.ReadFile(messagePath)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(message)
		signing := write("signing-plan-"+label, map[string]any{"schema": "ardents-initial-node-record-signing-plan-v1", "reference_time": now.Format(time.RFC3339), "record": public})
		signed := filepath.Join(parent, "signed-"+label)
		run("sign-initial-node-record", signing, messagePath, hex.EncodeToString(digest[:]), pair[1], signed)
		recordPath := filepath.Join(signed, "node-record.bin")
		record, err := os.ReadFile(recordPath)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(record, bundle.Epoch.Inputs[i]) {
			t.Fatal("State Record differs from genuine producer")
		}
		digest = sha256.Sum256(record)
		certificatePlan := write("certificate-plan-"+label, map[string]any{"schema": "ardents-node-certificate-plan-v1", "reference_time": now.Format(time.RFC3339), "record": public})
		output := filepath.Join(parent, "certificate-"+label)
		run("prepare-node-certificate", certificatePlan, recordPath, hex.EncodeToString(digest[:]), pair[1], output)
		pair[0] = filepath.Join(output, "node-certificate.pem")
		paths[n.NodeID] = pair
		cert, err := tls.LoadX509KeyPair(pair[0], pair[1])
		if err != nil {
			t.Fatal(err)
		}
		private := cert.PrivateKey.(ed25519.PrivateKey)
		t.Cleanup(func() { clear(private) })
		leaf, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			t.Fatal(err)
		}
		if leaf.CheckSignature(leaf.SignatureAlgorithm, leaf.RawTBSCertificate, leaf.Signature) != nil || !leaf.NotBefore.Equal(n.ValidFrom) || !leaf.NotAfter.Equal(n.ValidUntil) || leaf.IsCA || leaf.KeyUsage != x509.KeyUsageDigitalSignature || len(leaf.ExtKeyUsage) != 2 {
			t.Fatal("independent native leaf oracle")
		}
		// Confirm PEM contains only this public certificate, without private data.
		raw, err := os.ReadFile(pair[0])
		if err != nil {
			t.Fatal(err)
		}
		block, rest := pem.Decode(raw)
		if block == nil || len(rest) != 0 || block.Type != "CERTIFICATE" {
			t.Fatal("single public leaf")
		}
		certificates[n.NodeID] = cert
	}
	return paths
}

func TestNativeNodeProvisionedCertificateTCP(t *testing.T) {
	testNativeNodeForwarding(t, "certificate")
}
func TestNativeJoinProvisionedCertificateTCP(t *testing.T) { nativeJoinWorkload(t, "certificate") }
