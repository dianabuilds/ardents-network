package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetiredRouteV2ExecutionClosureIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		"internal/route/credential_relay_io.go",
		"internal/route/credential_relay_setup.go",
		"internal/route/credential_relay_test.go",
		"internal/route/endpoint_transit_attachment.go",
		"internal/route/endpoint_transit_attachment_test.go",
		"internal/route/endpoint_transit_binding.go",
		"internal/route/endpoint_transit_binding_test.go",
		"internal/route/entry_attachment.go",
		"internal/route/entry_attachment_test.go",
		"internal/route/entry_binding.go",
		"internal/route/entry_binding_test.go",
		"internal/route/introduction_control_io.go",
		"internal/route/introduction_control_io_test.go",
		"internal/route/introduction_outcome.go",
		"internal/route/introduction_outcome_io.go",
		"internal/route/introduction_outcome_test.go",
		"internal/route/introduction_slot_registration.go",
		"internal/route/introduction_slot_registration_test.go",
		"internal/route/native_attachment.go",
		"internal/route/native_attachment_test.go",
		"internal/route/node_binding.go",
		"internal/route/node_binding_test.go",
		"internal/route/route_binding_v1.go",
		"internal/route/route_binding_v1_test.go",
		"internal/route/transit_grant.go",
		"internal/route/transit_grant_test.go",
		"internal/route/wire_encoding.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired Route v2 execution closure file still exists: %s", relative)
		}
	}
}

func TestRouteV2RetirementPreservesRefusalAndHistoricalSealedGrammar(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	sealed := string(readProjectFile(t, root, "internal/route/sealed_introduction.go"))
	for _, retained := range []string{
		`Profile         = "ardents-interactive-route-v2"`,
		"func routeEnvelope(",
		"func routeBody(",
		"func DecodeSealedIntroduction(",
		"func OpenSealedIntroductionWith(",
	} {
		if !strings.Contains(sealed, retained) {
			t.Errorf("sealed Introduction owner lost retained historical declaration %q", retained)
		}
	}

	admission := string(readProjectFile(t, root, "internal/node/admission.go"))
	server := string(readProjectFile(t, root, "internal/node/duty_server.go"))
	if !strings.Contains(admission, "snapshot.Profile == route.Profile") ||
		!strings.Contains(server, "snapshot.Profile == route.Profile") {
		t.Error("Node lost the typed refusal of the retired v2 Route profile")
	}

	plan := string(readProjectFile(t, root, "cmd/ardents-node/node_config.go"))
	if strings.Contains(plan, "route.Profile") {
		t.Error("Node plan composition still selects the retired v2 Route profile")
	}

	carrier := string(readProjectFile(t, root, "internal/route/node_carrier.go"))
	if strings.Contains(carrier, "CarrierTCP") || strings.Contains(carrier, "CarrierQUIC") {
		t.Error("shared Carrier vocabulary still exports the retired v1 carrier constants")
	}

	store := string(readProjectFile(t, root, "internal/network/duty/store.go"))
	contract := string(readProjectFile(t, root, "internal/network/duty/contract.go"))
	if strings.Contains(store, "SpendTransitGrant") {
		t.Error("local-role store still exports the uncalled Transit Grant spend")
	}
	if !strings.Contains(contract, "TransitGrantSpends []transitGrantSpend") {
		t.Error("local-role durable v1 schema lost its persisted Transit Grant spend field")
	}
}
