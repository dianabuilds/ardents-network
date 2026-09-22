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
	entryAttachment := string(readProjectFile(t, root, "internal/route/entry_attachment.go"))
	if strings.Contains(entryAttachment, "ReadEntryOperation") {
		t.Error("Route still exposes the retired Initiator-only entry-operation dispatcher")
	}
}

func TestRetiredResponderEngineIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		"internal/node/responder_contract.go",
		"internal/node/responder_duty.go",
		"internal/node/responder_listener.go",
		"internal/node/responder_test.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired Responder engine file still exists: %s", relative)
		}
	}

	contracts := string(readProjectFile(t, root, "internal/node/contract.go"))
	if strings.Contains(contracts, "ResponderProfile") {
		t.Error("Node contract still exports the retired Responder profile")
	}
	server := string(readProjectFile(t, root, "internal/node/duty_server.go"))
	if strings.Contains(server, `case "responder":`) {
		t.Error("Node duty dispatch still starts the retired Responder engine")
	}
	identity := string(readProjectFile(t, root, "cmd/ardents-node/node_identity.go"))
	if strings.Contains(identity, "config.Responder") {
		t.Error("command composition still constructs the retired Responder engine profile")
	}
}

func TestRetiredIntroductionEngineIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		"internal/node/introduction_contract.go",
		"internal/node/introduction_duty.go",
		"internal/node/introduction_listener.go",
		"internal/node/introduction_test.go",
		"internal/node/transit_grant_admission.go",
		"internal/node/transit_grant_admission_test.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired Introduction engine file still exists: %s", relative)
		}
	}

	contracts := string(readProjectFile(t, root, "internal/node/contract.go"))
	if strings.Contains(contracts, "type IntroductionProfile ") || strings.Contains(contracts, "Introduction         IntroductionProfile") {
		t.Error("Node contract still exports the retired Introduction profile")
	}
	server := string(readProjectFile(t, root, "internal/node/duty_server.go"))
	if strings.Contains(server, `case "introduction":`) {
		t.Error("Node duty dispatch still starts the retired Introduction engine")
	}
	if !strings.Contains(server, "return startClosedIntroduction(config, snapshot)") {
		t.Error("Node duty dispatch lost the current closed Introduction branch")
	}
	nativeDuty := string(readProjectFile(t, root, "internal/node/native_duty.go"))
	if strings.Contains(nativeDuty, `case "introduction":`) {
		t.Error("Node duty validation still admits the retired Introduction engine")
	}
	identity := string(readProjectFile(t, root, "cmd/ardents-node/node_identity.go"))
	if strings.Contains(identity, "config.Introduction") {
		t.Error("command composition still constructs the retired Introduction engine profile")
	}
}

func TestRetiredOpenNodeLegIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	for _, relative := range []string{
		"internal/route/node_leg.go",
		"internal/route/node_leg_tcp.go",
		"internal/route/node_leg_quic.go",
		"internal/route/node_leg_test.go",
		"internal/route/node_leg_quic_test.go",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("retired Node-leg dial file still exists: %s", relative)
		}
	}

	binding := string(readProjectFile(t, root, "internal/route/node_binding.go"))
	if strings.Contains(binding, "ConfirmNodeLegBinding") {
		t.Error("Route still exports the retired Node-leg confirmation entrypoint")
	}
	carrier := string(readProjectFile(t, root, "internal/route/node_carrier_quic.go"))
	for _, retired := range []string{"openQUICNodeCarrier", "func (carrier *quicNodeCarrier) abort"} {
		if strings.Contains(carrier, retired) {
			t.Errorf("shared QUIC Carrier file still contains retired dial helper %q", retired)
		}
	}
}
