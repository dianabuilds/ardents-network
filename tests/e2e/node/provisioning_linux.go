//go:build linux && node_process_native

package node

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

	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// ProvisionNodeCertificates holds State, Admission and cohort authorities as fixtures. Every selected Node key,
// canonical prepared and signed initial Record, and certificate comes from the
// genuine command. Actual original paths are passed unchanged to Node runtime.
func ProvisionNodeCertificates(t testing.TB, spec *networkfixture.ClosedSpec, certificates map[[32]byte]tls.Certificate, ids ...[32]byte) map[[32]byte][2]string {
	t.Helper()
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	p, err := artifacts.OpenDirectoryChain(t.Context(), parent, nil)
	if err != nil {
		t.Fatal("invalid native environment", err)
	}
	accessErr := p.CheckRootAccess()
	if closeErr := p.Close(); accessErr != nil || closeErr != nil {
		t.Fatal("invalid root-controlled ancestors", accessErr, closeErr)
	}
	command := processfixture.PinnedCommand(t)
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
		key, err := artifacts.ParseEd25519Key(raw)
		clear(raw)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { clear(key) })
		found := false
		for j := range spec.Nodes {
			if spec.Nodes[j].NodeID == id {
				spec.Nodes[j].PrivateKey = key
				spec.Nodes[j].Generation = 1
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
	bundle, err := networkfixture.BuildClosed(*spec)
	if err != nil {
		t.Fatal(err)
	}
	spec.Nodes = bundle.Nodes
	for i, n := range bundle.Nodes {
		pair, selected := paths[n.NodeID]
		if !selected {
			continue
		}
		now := time.Now().UTC().Truncate(time.Second)
		public := map[string]any{"schema": "ardents-initial-node-record-plan-v1", "network_id": hex.EncodeToString(spec.NetworkID[:]), "node_id": hex.EncodeToString(n.NodeID[:]), "valid_from": n.ValidFrom.Format(time.RFC3339), "valid_until": n.ValidUntil.Format(time.RFC3339), "family": n.Family, "endpoint": n.Endpoint, "carrier": n.Carrier, "capacity": n.Capacity, "public_key": hex.EncodeToString(n.PrivateKey[32:])}
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
		private, ok := cert.PrivateKey.(ed25519.PrivateKey)
		if !ok {
			t.Fatal("native Node credential used a different key type")
		}
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
