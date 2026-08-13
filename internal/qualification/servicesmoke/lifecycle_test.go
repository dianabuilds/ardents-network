package servicesmoke

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPrepareCreatesStableTargetChangingInstanceAndCleansPrivateRoot(t *testing.T) {
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
	if value, err := topologyReceipt(good); err != nil || len(value) != 8 {
		t.Fatalf("isolated topology rejected: value=%v err=%v", value, err)
	}
	bad := bytes.Replace(good, []byte("  client-app:\n    network_mode: none"),
		[]byte("  client-app:\n    network_mode: none\n    volumes: [client_route]"), 1)
	if _, err := topologyReceipt(bad); err == nil {
		t.Fatal("application Route visibility was accepted")
	}
}
