package servicesmoke

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPrepareCreatesStableTargetChangingInstanceAndCleansPrivateRoot(t *testing.T) {
	original := generateInstance
	generateInstance = func(_ string, path string) ([32]byte, error) {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return [32]byte{}, err
		}
		if err := os.WriteFile(path, []byte(hex.EncodeToString(private)), 0o600); err != nil {
			return [32]byte{}, err
		}
		var value [32]byte
		copy(value[:], public)
		return value, nil
	}
	defer func() { generateInstance = original }()
	parent := t.TempDir()
	fixtureRoot := filepath.Join(parent, "private")
	evidenceRoot := filepath.Join(parent, "evidence")
	value, err := prepare(Config{FixtureRoot: fixtureRoot, EvidenceRoot: evidenceRoot,
		ComposeFile: filepath.Join(parent, "compose.yaml"), SourceRoot: parent, Duration: 10 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if value.target == [32]byte{} || value.credentials[0].Target != value.target ||
		value.credentials[1].Target != value.target || value.credentials[0].InstancePublic == value.credentials[1].InstancePublic {
		t.Fatal("prepared migration does not preserve Target while changing Instance Key")
	}
	if err := removePrivateFixture(fixtureRoot); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fixtureRoot); !os.IsNotExist(err) {
		t.Fatal("private fixture remained after cleanup")
	}
}

func TestPrivateCleanupRefusesUnownedRoot(t *testing.T) {
	root := t.TempDir()
	if err := removePrivateFixture(root); err == nil {
		t.Fatal("cleanup accepted a root without the exact ownership marker")
	}
}

func TestTopologyReceiptRequiresApplicationAndEndpointIsolation(t *testing.T) {
	good := []byte("services:\n  client-app:\n    network_mode: none\n  publisher-app:\n    network_mode: none\n" +
		"  client-endpoint:\n    network_mode: none\n  publisher-endpoint:\n    network_mode: none\nnetworks:\n  route_net:\n    internal: true\n")
	if value, err := topologyReceipt(good); err != nil || len(value) != 6 {
		t.Fatalf("isolated topology rejected: value=%v err=%v", value, err)
	}
	bad := bytes.Replace(good, []byte("  client-app:\n    network_mode: none"),
		[]byte("  client-app:\n    network_mode: none\n    volumes: [client_route]"), 1)
	if _, err := topologyReceipt(bad); err == nil {
		t.Fatal("application Route visibility was accepted")
	}
}

func TestContainerIDExtractionIgnoresComposeProgress(t *testing.T) {
	identity := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	raw := []byte(" Container fixture Creating\n Container fixture Created\n" + identity + "\n")
	if got := containerIDFromOutput(raw); got != identity {
		t.Fatalf("container identity=%q, want %q", got, identity)
	}
	if got := containerIDFromOutput([]byte("created without identity")); got != "" {
		t.Fatalf("malformed output produced identity %q", got)
	}
}

func TestJSONLineSeparatesMachineReceiptFromDiagnostics(t *testing.T) {
	raw := []byte("invalid\n{\"schema\":\"verdict\",\"verdict\":\"invalid\"}\n")
	want := "{\"schema\":\"verdict\",\"verdict\":\"invalid\"}\n"
	if got := string(jsonLine(raw, "verdict")); got != want {
		t.Fatalf("JSON receipt=%q, want %q", got, want)
	}
}
