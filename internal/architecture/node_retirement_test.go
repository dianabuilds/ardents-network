package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetiredInitiatorEngineIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		"internal/node/entry_admission.go",
		"internal/node/entry_admission_test.go",
		"internal/node/initiator_contract.go",
		"internal/node/initiator_duty.go",
		"internal/node/initiator_lifecycle_test.go",
		"internal/node/initiator_listener.go",
		"internal/node/initiator_relay.go",
		"internal/node/initiator_test.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired Initiator engine file still exists: %s", relative)
		}
	}

	contracts := string(readProjectFile(t, root, "internal/node/contract.go"))
	if strings.Contains(contracts, "InitiatorProfile") {
		t.Error("Node contract still exports the retired Initiator profile")
	}
	server := string(readProjectFile(t, root, "internal/node/duty_server.go"))
	if strings.Contains(server, `case "initiator":`) {
		t.Error("Node duty dispatch still starts the retired Initiator engine")
	}
	identity := string(readProjectFile(t, root, "cmd/ardents-node/node_identity.go"))
	if strings.Contains(identity, "config.Initiator") {
		t.Error("command composition still constructs the retired Initiator engine profile")
	}
	relayIO := string(readProjectFile(t, root, "internal/route/relay_setup_io.go"))
	if strings.Contains(relayIO, "ReadEntryOperation") {
		t.Error("Route still exposes the retired Initiator-only entry-operation dispatcher")
	}
}
