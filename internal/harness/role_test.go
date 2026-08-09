package harness

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestSmokeRolesSeeOnlyTheirConfiguredPeer(t *testing.T) {
	t.Parallel()
	runID := "20260809T120000Z-smoke"
	alphaAddress := reserveAddress(t)
	betaAddress := reserveAddress(t)
	root := t.TempDir()
	alphaConfig := filepath.Join(root, "alpha.json")
	betaConfig := filepath.Join(root, "beta.json")
	alphaEvidence := filepath.Join(root, "alpha-evidence")
	betaEvidence := filepath.Join(root, "beta-evidence")
	for _, directory := range []string{alphaEvidence, betaEvidence} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeRoleConfig(t, alphaConfig, smokeRoleConfig{
		SchemaVersion: smokeRoleSchema, RunID: runID, Role: "alpha", ListenAddress: alphaAddress,
		PeerRole: "beta", PeerAddress: betaAddress,
	})
	writeRoleConfig(t, betaConfig, smokeRoleConfig{
		SchemaVersion: smokeRoleSchema, RunID: runID, Role: "beta", ListenAddress: betaAddress,
		PeerRole: "alpha", PeerAddress: alphaAddress,
	})

	errors := make(chan error, 2)
	go func() { errors <- RunRole(alphaConfig, alphaEvidence) }()
	go func() { errors <- RunRole(betaConfig, betaEvidence) }()
	for range 2 {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
	for _, evidenceDir := range []string{alphaEvidence, betaEvidence} {
		var result smokeRoleResult
		loadJSON(t, filepath.Join(evidenceDir, "result.json"), &result)
		if result.Status != "passed" || len(result.ObservedPeers) != 2 || result.ObservedPeers[0] != result.ObservedPeers[1] {
			t.Fatalf("role result = %#v", result)
		}
	}
}

func TestSmokeRoleRejectsUnexpectedKnowledge(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	configPath := filepath.Join(root, "role.json")
	evidenceDir := filepath.Join(root, "evidence")
	if err := os.Mkdir(evidenceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`{"schema_version":"carrier-lab-smoke-role/v1","run_id":"run","role":"alpha","peer_role":"beta","listen_address":":1","peer_address":"beta:1","target":"forbidden"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RunRole(configPath, evidenceDir); err == nil {
		t.Fatal("role config accepted an undeclared knowledge field")
	}
}

func reserveAddress(t *testing.T) string {
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

func writeRoleConfig(t *testing.T, path string, config smokeRoleConfig) {
	t.Helper()
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func loadJSON(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
}
