package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetiredTransitIssuerReceivingEngineIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		"internal/node/transit_grant_signer.go",
		"internal/node/transit_grant_signer_test.go",
		"internal/node/transit_issuer_duty.go",
		"internal/node/transit_issuer_lifecycle_test.go",
		"internal/node/transit_issuer_listener.go",
		"internal/route/credential/client.go",
		"internal/route/credential/durable_issuer_test.go",
		"internal/route/credential/issuer.go",
		"internal/route/credential/issuer_root.go",
		"internal/route/credential/issuer_root_store.go",
		"internal/route/credential/issuer_root_test.go",
		"internal/route/credential/issuer_state_duty_test.go",
		"internal/route/credential/issuer_test.go",
		"internal/route/credential/message.go",
		"internal/route/credential/profile.go",
		"internal/route/credential/root_issuer.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired Transit issuer engine file still exists: %s", relative)
		}
	}

	contracts := string(readProjectFile(t, root, "internal/node/contract.go"))
	if strings.Contains(contracts, "TransitIssuerProfile") || strings.Contains(contracts, "TransitGrantSigner") {
		t.Error("Node contract still exports the retired Transit issuer engine")
	}
	server := string(readProjectFile(t, root, "internal/node/duty_server.go"))
	if strings.Contains(server, `case "transit-issuance":`) || strings.Contains(server, "startTransitIssuer") {
		t.Error("Node duty dispatch still starts the retired Transit issuer engine")
	}
	identity := string(readProjectFile(t, root, "cmd/ardents-node/node_identity.go"))
	if strings.Contains(identity, "config.TransitIssuer") {
		t.Error("command composition still constructs the retired Transit issuer engine profile")
	}
}

func TestTransitIssuerRetirementPreservesRefusalAndClosedIssuer(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	command := string(readProjectFile(t, root, "cmd/ardents-node/issuer_initialize.go"))
	plan := string(readProjectFile(t, root, "cmd/ardents-node/node_config.go"))
	if !strings.Contains(command, `errors.New("old Transit issuer start is retired")`) ||
		!strings.Contains(plan, `TransitIssuer`) || !strings.Contains(plan, `return "transit_issuer"`) {
		t.Error("command boundary lost the stable old Transit issuer refusal")
	}

	for _, retained := range []struct {
		path        string
		declaration string
	}{
		{"internal/route/credential/closed_token_issuer.go", "type ClosedTokenIssuer struct"},
		{"internal/route/credential/closed_token_issuer_ledger.go", "type closedTokenIssuerLedger struct"},
	} {
		content := string(readProjectFile(t, root, retained.path))
		if !strings.Contains(content, retained.declaration) {
			t.Errorf("%s lost retained declaration %q", retained.path, retained.declaration)
		}
	}
}
