//go:build linux && network_signing_native

package source

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// CertificateArguments prepares genuine initial keys and a signed Record using the compiled CLI.
func CertificateArguments(t *testing.T, command, parent string) []string {
	t.Helper()
	keyPlan, keyRoot := InitialKeyArguments(t, parent, 1)
	run := func(args ...string) {
		t.Helper()
		if raw, err := exec.CommandContext(t.Context(), command, args...).CombinedOutput(); err != nil {
			t.Fatal("genuine provisioning command", err, string(raw))
		}
	}
	run("network", "prepare-initial-keys", keyPlan, keyRoot)
	keys := VerifyInitialKeys(t, keyRoot, 1, nil)
	networkID, nodeID := [32]byte{1}, [32]byte{2}
	p := map[string]any{
		"schema":     "ardents-initial-node-record-plan-v1",
		"network_id": hex.EncodeToString(networkID[:]), "node_id": hex.EncodeToString(nodeID[:]),
		"family": "family-one", "endpoint": "127.0.0.1:4401", "carrier": "ardents-carrier-tcp-tls-v2",
		"capacity": 3, "public_key": hex.EncodeToString(keys[1][32:]),
	}
	now := time.Now().UTC().Truncate(time.Second)
	p["valid_from"], p["valid_until"] = now.Add(-time.Minute).Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339)
	plan, prepared, recordPlan, signedRoot := filepath.Join(parent, "record-plan"), filepath.Join(parent, "prepared"), filepath.Join(parent, "signing-plan"), filepath.Join(parent, "signed")
	if err := os.WriteFile(plan, marshalCertificateInput(t, p), 0600); err != nil {
		t.Fatal(err)
	}
	run("network", "prepare-initial-node-record", plan, prepared)
	message, err := os.ReadFile(filepath.Join(prepared, "node-record.signing-input"))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(message)
	if err := os.WriteFile(recordPlan, marshalCertificateInput(t, map[string]any{"schema": "ardents-initial-node-record-signing-plan-v1", "reference_time": now.Format(time.RFC3339), "record": p}), 0600); err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(keyRoot, "node-00-key.pem")
	run("network", "sign-initial-node-record", recordPlan, filepath.Join(prepared, "node-record.signing-input"), hex.EncodeToString(digest[:]), key, signedRoot)
	record := filepath.Join(signedRoot, "node-record.bin")
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	digest = sha256.Sum256(raw)
	declaration := filepath.Join(parent, "certificate-plan")
	if err := os.WriteFile(declaration, marshalCertificateInput(t, map[string]any{"schema": "ardents-node-certificate-plan-v1", "reference_time": now.Format(time.RFC3339Nano), "record": p}), 0600); err != nil {
		t.Fatal(err)
	}
	return []string{"prepare-node-certificate", declaration, record, hex.EncodeToString(digest[:]), key, filepath.Join(parent, "certificate")}
}

func marshalCertificateInput(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
