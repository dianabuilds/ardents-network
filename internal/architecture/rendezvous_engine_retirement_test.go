package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetiredRendezvousReceivingEngineIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		"cmd/ardents-node/rendezvous_listen_test.go",
		"internal/node/native_duty.go",
		"internal/node/rendezvous_bind.go",
		"internal/node/rendezvous_bind_test.go",
		"internal/node/rendezvous_contract.go",
		"internal/node/rendezvous_duty.go",
		"internal/node/rendezvous_fixture_test.go",
		"internal/node/rendezvous_lifecycle_test.go",
		"internal/node/rendezvous_listener.go",
		"internal/node/rendezvous_pairing.go",
		"internal/node/rendezvous_pressure_usage_test.go",
		"internal/node/rendezvous_resource_profile_test.go",
		"internal/node/rendezvous_test.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired Rendezvous engine file still exists: %s", relative)
		}
	}

	contracts := string(readProjectFile(t, root, "internal/node/contract.go"))
	if strings.Contains(contracts, "type RendezvousProfile ") || strings.Contains(contracts, "Rendezvous           RendezvousProfile") {
		t.Error("Node contract still exports the retired Rendezvous engine profile")
	}
	server := string(readProjectFile(t, root, "internal/node/duty_server.go"))
	if strings.Contains(server, "startRendezvous") || strings.Contains(server, "rendezvousDuty") || strings.Contains(server, "rendezvousPressureUsage") {
		t.Error("Node duty dispatch still starts or accounts for the retired Rendezvous engine")
	}
	identity := string(readProjectFile(t, root, "cmd/ardents-node/node_identity.go"))
	if strings.Contains(identity, "config.Rendezvous") {
		t.Error("command composition still constructs the retired Rendezvous engine profile")
	}
}

func TestRendezvousEngineRetirementPreservesRefusalAndClosedDuties(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	plan := string(readProjectFile(t, root, "cmd/ardents-node/node_config.go"))
	if !strings.Contains(plan, "Rendezvous              *rendezvousPlan") || !strings.Contains(plan, `return "rendezvous"`) {
		t.Error("command boundary lost the stable old Rendezvous reservation refusal")
	}

	server := string(readProjectFile(t, root, "internal/node/duty_server.go"))
	for _, start := range []string{
		"return startClosedIssuer(config, snapshot)",
		"return startClosedForwarding(config, snapshot)",
		"return startClosedResolution(config, snapshot)",
		"return startClosedIntroduction(config, snapshot)",
		"return startClosedDataJoin(config, snapshot)",
	} {
		if !strings.Contains(server, start) {
			t.Errorf("Node duty dispatch lost current closed branch %q", start)
		}
	}
	shared := string(readProjectFile(t, root, "internal/node/closed_listen_bind.go"))
	if !strings.Contains(shared, "func literalNodeEndpoint(") {
		t.Error("closed listeners lost their shared literal-endpoint validator")
	}
}
