//go:build referencec2 && (h4_3b_multihost || h4_8_a11)

package service_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
)

type h48A11CarrierRelaySnapshot struct {
	AcceptedBridges        uint64 `json:"accepted_bridges"`
	AcceptedAfterReset     uint64 `json:"accepted_after_reset"`
	ClientToNodeBytes      uint64 `json:"client_to_node_bytes"`
	NodeToClientBytes      uint64 `json:"node_to_client_bytes"`
	ActiveBridges          uint32 `json:"active_bridges"`
	PeakActiveBridges      uint32 `json:"peak_active_bridges"`
	ResetCount             uint32 `json:"reset_count"`
	ResetBridges           uint32 `json:"reset_bridges"`
	ActiveBefore           uint32 `json:"active_before"`
	SelectedBridgeID       uint64 `json:"selected_bridge_id"`
	ListenerLiveAfterReset bool   `json:"listener_live_after_reset"`
}

type h48A11FaultInjection struct {
	Schema           string `json:"schema"`
	Fault            string `json:"fault"`
	TargetRole       string `json:"target_role"`
	Signal           string `json:"signal"`
	ReadyMarker      string `json:"ready_marker"`
	TargetPID        int    `json:"target_pid"`
	WarmupCycles     uint32 `json:"warmup_cycles"`
	ProductNodeLive  bool   `json:"product_node_live"`
	CarrierRelayLive bool   `json:"carrier_relay_live"`
}

type h48A11KillReceipt struct {
	Schema                   string `json:"schema"`
	Fault                    string `json:"fault"`
	Signal                   string `json:"signal"`
	PID                      int    `json:"pid"`
	ExitStatus               int    `json:"exit_status"`
	PublisherLiveBefore      bool   `json:"publisher_live_before"`
	PublisherAppLiveBefore   bool   `json:"publisher_app_live_before"`
	RendezvousNodeLiveBefore bool   `json:"rendezvous_node_live_before"`
	CarrierRelayLiveBefore   bool   `json:"carrier_relay_live_before"`
	InjectedAfterReady       bool   `json:"injected_after_ready"`
}

type h48A11ApplicationResetReceipt struct {
	Schema                   string `json:"schema"`
	Fault                    string `json:"fault"`
	Action                   string `json:"action"`
	PID                      int    `json:"pid"`
	PublisherLiveBefore      bool   `json:"publisher_live_before"`
	PublisherAppLiveBefore   bool   `json:"publisher_app_live_before"`
	RendezvousNodeLiveBefore bool   `json:"rendezvous_node_live_before"`
	CarrierRelayLiveBefore   bool   `json:"carrier_relay_live_before"`
	TransitRolesLiveBefore   bool   `json:"transit_roles_live_before"`
	InjectedAfterReady       bool   `json:"injected_after_ready"`
}

func h48A11AssertProductTransitEvidence(t *testing.T, remote h43RemoteC2, scenario referenceC2Scenario, user commandResult) {
	t.Helper()
	var fixture struct {
		Network, Digest string
		Epoch           uint64
		Rendezvous      struct{ Endpoint string }
	}
	h48A11ReadRemoteJSON(t, remote, "reference-c2.json", &fixture)
	var topology h48A11Topology
	h48A11ReadRemoteJSON(t, remote, "topology.json", &topology)
	if topology.Schema != "ardents-h4-8-a11-topology-v1" || topology.StateRendezvousEndpoint != fixture.Rendezvous.Endpoint ||
		topology.CarrierRelayListen != fixture.Rendezvous.Endpoint || topology.ProductRendezvousListen != topology.CarrierRelayTarget ||
		topology.RendezvousImplementation != "ardents-node" || topology.FixtureRendezvousStarted || topology.AlphaRelayPurpose != "private-resolution-only" {
		t.Fatalf("A11 retained topology = %+v / fixture endpoint %q", topology, fixture.Rendezvous.Endpoint)
	}
	if _, err := remote.readFile(t, remote.environment.remoteDirectory+"/rendezvous.log"); err == nil {
		t.Fatal("A11 started the forbidden fixture Rendezvous")
	}
	nodePID := h48A11RemotePID(t, remote, "rendezvous-node.pid")
	relayPID := h48A11RemotePID(t, remote, "carrier-relay.pid")
	publisherPID := h48A11RemotePID(t, remote, "publisher.pid")
	publisherAppPID := h48A11RemotePID(t, remote, "publisher-app.pid")
	var ready struct {
		Schema, Listen, Target string
		PID                    int
	}
	h48A11ReadRemoteJSON(t, remote, "carrier-relay-ready.json", &ready)
	if ready.Schema != "ardents-h4-8-a11-carrier-relay-ready-v1" || ready.PID != relayPID ||
		ready.Listen != topology.CarrierRelayListen || ready.Target != topology.CarrierRelayTarget {
		t.Fatalf("A11 Carrier relay ready receipt = %+v", ready)
	}
	h48A11AssertProductLifecycle(t, remote, scenario, fixture.Epoch)
	relay := h43RemoteResult(t, remote, "carrier-relay").CarrierRelay
	if relay == nil || relay.AcceptedBridges != 2 || relay.PeakActiveBridges != 2 || relay.ActiveBridges != 0 ||
		relay.ClientToNodeBytes == 0 || relay.NodeToClientBytes == 0 {
		t.Fatalf("A11 Carrier relay terminal counters = %+v", relay)
	}
	switch {
	case scenario.publisherTerminal == referenceC2PublisherApplicationReset:
		fault := h48A11ReadFaultInjection(t, remote)
		if fault.Fault != "publisher-application-loss" || fault.TargetRole != "publisher-app" || fault.TargetPID != publisherAppPID ||
			fault.Signal != "RESET" || fault.ReadyMarker != "publisher-application-fault-ready" || fault.WarmupCycles != scenario.dynamicWorkload.Cycles {
			t.Fatalf("A11 Publisher Application fault injection = %+v", fault)
		}
		var reset h48A11ApplicationResetReceipt
		h48A11ReadRemoteJSON(t, remote, "publisher-application-reset.json", &reset)
		if reset.Schema != "ardents-h4-8-a11-publisher-application-reset-v1" || reset.Fault != "publisher-application-loss" ||
			reset.Action != "RESET" || reset.PID != publisherAppPID || !reset.PublisherLiveBefore || !reset.PublisherAppLiveBefore ||
			!reset.RendezvousNodeLiveBefore || !reset.CarrierRelayLiveBefore || !reset.TransitRolesLiveBefore || !reset.InjectedAfterReady {
			t.Fatalf("A11 Publisher Application reset receipt = %+v", reset)
		}
		if relay.ResetCount != 0 || relay.ResetBridges != 0 || relay.ActiveBefore != 0 || relay.SelectedBridgeID != 0 ||
			relay.AcceptedAfterReset != 0 || relay.ListenerLiveAfterReset {
			t.Fatalf("Publisher Application fault substituted a Carrier reset: %+v", relay)
		}
	case scenario.publisherTerminal == referenceC2PublisherEndpointStop:
		fault := h48A11ReadFaultInjection(t, remote)
		if fault.Fault != "publisher-endpoint-loss" || fault.TargetRole != "publisher" || fault.TargetPID != publisherPID || fault.Signal != "KILL" ||
			fault.ReadyMarker != "publisher-crash-ready" || fault.WarmupCycles != scenario.dynamicWorkload.Cycles {
			t.Fatalf("A11 Publisher Endpoint fault injection = %+v", fault)
		}
		var killed h48A11KillReceipt
		h48A11ReadRemoteJSON(t, remote, "publisher-endpoint-kill.json", &killed)
		h48A11AssertKillReceipt(t, killed, "ardents-h4-8-a11-publisher-endpoint-kill-v1", "publisher-endpoint-loss", fault.TargetPID)
		if relay.ResetCount != 0 || relay.ResetBridges != 0 || relay.ActiveBefore != 0 || relay.SelectedBridgeID != 0 ||
			relay.AcceptedAfterReset != 0 || relay.ListenerLiveAfterReset {
			t.Fatalf("Publisher Endpoint fault substituted a Carrier reset: %+v", relay)
		}
	case scenario.transitFault == referenceC2TransitFaultCarrierLoss:
		fault := h48A11ReadFaultInjection(t, remote)
		if fault.Fault != "carrier-loss" || fault.TargetRole != "carrier-relay" || fault.TargetPID != relayPID || fault.Signal != "RESET" ||
			fault.ReadyMarker != "transit-fault-ready" || fault.WarmupCycles != scenario.dynamicWorkload.Cycles {
			t.Fatalf("A11 Carrier fault injection = %+v", fault)
		}
		var reset struct {
			Schema           string `json:"schema"`
			ResetCount       uint32 `json:"reset_count"`
			ResetBridges     uint32 `json:"reset_bridges"`
			ActiveBefore     uint32 `json:"active_before"`
			SelectedBridgeID uint64 `json:"selected_bridge_id"`
			ActiveBridges    uint32 `json:"active_bridges"`
			ListenerLive     bool   `json:"listener_live"`
		}
		h48A11ReadRemoteJSON(t, remote, "carrier-relay-reset.json", &reset)
		if reset.Schema != "ardents-h4-8-a11-carrier-relay-reset-v1" || reset.ResetCount != 1 || reset.ResetBridges != 2 ||
			reset.ActiveBefore != 2 || reset.SelectedBridgeID != 0 || reset.ActiveBridges != 0 || !reset.ListenerLive ||
			relay.ResetCount != 1 || relay.ResetBridges != 2 || relay.ActiveBefore != 2 ||
			relay.SelectedBridgeID != 0 || relay.AcceptedAfterReset != 0 || !relay.ListenerLiveAfterReset {
			t.Fatalf("A11 Carrier reset = %+v / terminal %+v", reset, relay)
		}
	case scenario.transitFault == referenceC2TransitFaultProductNodeLoss:
		fault := h48A11ReadFaultInjection(t, remote)
		if fault.Fault != "product-node-loss" || fault.TargetRole != "rendezvous-node" || fault.TargetPID != nodePID || fault.Signal != "KILL" ||
			fault.ReadyMarker != "transit-fault-ready" || fault.WarmupCycles != scenario.dynamicWorkload.Cycles {
			t.Fatalf("A11 product Node fault injection = %+v", fault)
		}
		var killed h48A11KillReceipt
		h48A11ReadRemoteJSON(t, remote, "rendezvous-node-kill.json", &killed)
		h48A11AssertKillReceipt(t, killed, "ardents-h4-8-a11-rendezvous-kill-v1", "product-node-loss", nodePID)
		if relay.ResetCount != 0 || relay.ResetBridges != 0 || relay.ActiveBefore != 0 || relay.SelectedBridgeID != 0 || relay.AcceptedAfterReset != 0 {
			t.Fatalf("product Node fault substituted a Carrier reset: %+v", relay)
		}
	default:
		if relay.ResetCount != 0 || relay.ResetBridges != 0 || relay.ActiveBefore != 0 || relay.SelectedBridgeID != 0 ||
			relay.AcceptedAfterReset != 0 || relay.ListenerLiveAfterReset {
			t.Fatalf("ordinary A11 topology injected a Carrier reset: %+v", relay)
		}
	}
	h48A11AssertRuntimeIdentity(t, remote, scenario, user)
}

func h48A11AssertProductLifecycle(t *testing.T, remote h43RemoteC2, scenario referenceC2Scenario, epoch uint64) {
	t.Helper()
	raw, err := remote.readFile(t, remote.environment.remoteDirectory+"/rendezvous-node.log")
	if err != nil {
		t.Fatal(err)
	}
	ready, draining, withdrawn := false, false, false
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var event referenceC2ProductNodeEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil || event.Schema != "ardents-node-event-v1" {
			t.Fatalf("A11 product Rendezvous lifecycle line = %q / %v", line, err)
		}
		switch event.State {
		case "READY":
			ready = event.Assignment == "rendezvous" && event.CarrierProfile == "ardents-carrier-tcp-tls-v1" && event.Epoch == epoch
		case "DRAINING":
			draining = ready
		case "WITHDRAWN":
			withdrawn = draining
		}
	}
	if !ready || scenario.transitFault == referenceC2TransitFaultProductNodeLoss && (draining || withdrawn) ||
		scenario.transitFault != referenceC2TransitFaultProductNodeLoss && (!draining || !withdrawn) {
		t.Fatalf("A11 product Rendezvous lifecycle ready=%t draining=%t withdrawn=%t", ready, draining, withdrawn)
	}
}

func h48A11AssertRuntimeIdentity(t *testing.T, remote h43RemoteC2, scenario referenceC2Scenario, user commandResult) {
	t.Helper()
	userRuntime := h48A11ProcessRuntime(t, user.output)
	assertReferenceC2EndpointRuntime(t, scenario, "user", userRuntime)
	if scenario.publisherTerminal == referenceC2PublisherEndpointStop {
		return
	}
	publisherRuntime := h43RemoteResult(t, remote, "publisher").Runtime
	assertReferenceC2EndpointRuntime(t, scenario, "publisher", publisherRuntime)
	if publisherRuntime.AuthenticatedTarget != userRuntime.AuthenticatedTarget || publisherRuntime.Generation != userRuntime.Generation ||
		publisherRuntime.RouteGeneration != userRuntime.RouteGeneration || publisherRuntime.RecoveryCount != userRuntime.RecoveryCount || publisherRuntime.Class != userRuntime.Class {
		t.Fatalf("A11 Publisher/User authenticated runtime mismatch: publisher=%+v user=%+v", publisherRuntime, userRuntime)
	}
}

func h48A11ProcessRuntime(t *testing.T, output []byte) *endpointapi.RuntimeResult {
	t.Helper()
	line := strings.TrimSpace(string(output))
	if index := strings.LastIndex(line, "\n"); index >= 0 {
		line = line[index+1:]
	}
	var result struct{ Runtime *endpointapi.RuntimeResult }
	if err := json.Unmarshal([]byte(line), &result); err != nil || result.Runtime == nil {
		t.Fatalf("decode A11 User runtime: %v / %q", err, output)
	}
	return result.Runtime
}

func h48A11ReadFaultInjection(t *testing.T, remote h43RemoteC2) h48A11FaultInjection {
	t.Helper()
	var receipt h48A11FaultInjection
	h48A11ReadRemoteJSON(t, remote, "fault-injection.json", &receipt)
	if receipt.Schema != "ardents-h4-8-a11-fault-injection-v1" || receipt.TargetPID <= 1 || !receipt.ProductNodeLive || !receipt.CarrierRelayLive {
		t.Fatalf("A11 fault injection receipt = %+v", receipt)
	}
	return receipt
}

func h48A11AssertKillReceipt(t *testing.T, receipt h48A11KillReceipt, schema, fault string, pid int) {
	t.Helper()
	if receipt.Schema != schema || receipt.Fault != fault || receipt.PID != pid || receipt.Signal != "KILL" || receipt.ExitStatus != 137 ||
		!receipt.PublisherLiveBefore || !receipt.PublisherAppLiveBefore || !receipt.RendezvousNodeLiveBefore || !receipt.CarrierRelayLiveBefore || !receipt.InjectedAfterReady {
		t.Fatalf("A11 kill receipt = %+v", receipt)
	}
}

func h48A11RemotePID(t *testing.T, remote h43RemoteC2, name string) int {
	t.Helper()
	raw, err := remote.readFile(t, remote.environment.remoteDirectory+"/"+name)
	pid, parseErr := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || parseErr != nil || pid <= 1 {
		t.Fatalf("A11 %s = %q / %v / %v", name, raw, err, parseErr)
	}
	return pid
}

func h48A11ReadRemoteJSON(t *testing.T, remote h43RemoteC2, name string, value any) {
	t.Helper()
	raw, err := remote.readFile(t, remote.environment.remoteDirectory+"/"+name)
	if err != nil || len(raw) > 64<<10 || json.Unmarshal(raw, value) != nil {
		t.Fatalf("A11 retained %s is invalid: %v / %q", name, err, raw)
	}
}
