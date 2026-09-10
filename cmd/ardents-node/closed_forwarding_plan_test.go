package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestNodePlanConnectsClosedForwardingToStateOwnedRuntime(t *testing.T) {
	plan := forwardingNodePlan(t)
	path := writeForwardingNodePlan(t, plan)
	runtime, err := readNodePlan(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.state.AcceptedProfile != route.ClosedRouteProfile ||
		!bytes.Equal(runtime.state.ClosedProfileAuthority, bytes.Repeat([]byte{0x12}, 32)) {
		t.Fatal("forwarding did not retain exact pinned closed State")
	}
	local := runtime.node.ClosedForwarding
	if local.Root != plan.ClosedForwarding.Root || local.ConnectionLimit != 2 ||
		local.DrainTimeout != 2*time.Second || local.Certificate.PrivateKey == nil ||
		runtime.node.Probe.ListenAddress != "" || runtime.node.ClosedIssuer.Root != "" {
		t.Fatal("forwarding reservation did not reach the Node owner unchanged")
	}
	// This is a forwarding reservation, never a request to start a signer.
	if err := runIssuerNode(t.Context(), path, new(bytes.Buffer)); err == nil {
		t.Fatal("issuer command accepted a forwarding-only reservation")
	}
}
func TestNodePlanRefusesMixedOrUnpinnedClosedForwarding(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*nodePlan)
	}{
		{"missing pin", func(plan *nodePlan) { plan.ClosedProfileAuthority = "" }},
		{"foreign pin", func(plan *nodePlan) { plan.ClosedProfileAuthority = strings.Repeat("ff", 32) }},
		{"issuer", func(plan *nodePlan) { plan.ClosedIssuer = &closedIssuerPlan{} }},
		{"rendezvous", func(plan *nodePlan) { plan.Rendezvous = &rendezvousPlan{} }},
		{"initiator", func(plan *nodePlan) { plan.Initiator = &initiatorPlan{} }},
		{"introduction", func(plan *nodePlan) { plan.Introduction = &introductionPlan{} }},
		{"responder", func(plan *nodePlan) { plan.Responder = &responderPlan{} }},
		{"transit", func(plan *nodePlan) { plan.TransitIssuer = &transitIssuerPlan{} }},
		{"resource profile", func(plan *nodePlan) { plan.NodeResourceProfile = "ardents-rendezvous-dedicated-host-v1" }},
		{"pin without duty", func(plan *nodePlan) { plan.ClosedForwarding = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := forwardingNodePlan(t)
			test.change(&plan)
			if _, err := readNodePlan(writeForwardingNodePlan(t, plan)); err == nil {
				t.Fatal("ambiguous or unpinned forwarding plan accepted")
			}
		})
	}
}
func TestClosedForwardingPlanCannotSelectPeerOrDuty(t *testing.T) {
	plan := forwardingNodePlan(t)
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"endpoint", "peer", "assignment", "profile", "receiver_node_id"} {
		t.Run(key, func(t *testing.T) {
			var stanza map[string]any
			encoded, err := json.Marshal(plan.ClosedForwarding)
			if err != nil || json.Unmarshal(encoded, &stanza) != nil {
				t.Fatal("encode local reservation")
			}
			stanza[key] = "caller-selected"
			fields["closed_forwarding"], err = json.Marshal(stanza)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "node.json")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := readNodePlan(path); err == nil {
				t.Fatal("forwarding plan accepted remote authority facts")
			}
		})
	}
}
func forwardingNodePlan(t *testing.T) nodePlan {
	t.Helper()
	certificate, key, nodeID := writeRendezvousListenCredential(t)
	rootA := writeNodeProfileInput(t, "a.pem", "source A root")
	rootB := writeNodeProfileInput(t, "b.pem", "source B root")
	return nodePlan{
		sourceServerPlan: sourceServerPlan{
			Schema: "ardents-node-plan-v1", StateRoot: t.TempDir(), LocalRoleStateRoot: t.TempDir(),
			NetworkID: strings.Repeat("11", 32), AuthorityPublic: []string{strings.Repeat("12", 32)}, Threshold: 1,
			ServerCertificate: certificate, ServerKey: key,
		},
		NodeID: nodeID, IdentityKey: key, ClockObservationFile: certificate, OrderSeed: strings.Repeat("13", 32),
		SourceClientCertificate: certificate, SourceClientKey: key,
		Sources: []nodeSource{
			{Address: "192.0.2.10:48010", ServerName: "a.test", Identity: strings.Repeat("14", 32), Family: "a", EndpointHandle: "a", RootCA: rootA, LeafKeyDigest: strings.Repeat("15", 32)},
			{Address: "192.0.2.11:48011", ServerName: "b.test", Identity: strings.Repeat("16", 32), Family: "b", EndpointHandle: "b", RootCA: rootB, LeafKeyDigest: strings.Repeat("17", 32)},
		},
		ClosedProfileAuthority: strings.Repeat("12", 32),
		ClosedForwarding:       &closedForwardingPlan{Root: t.TempDir(), ConnectionLimit: 2, DrainTimeoutMS: 2000},
	}
}
func writeForwardingNodePlan(t *testing.T, plan nodePlan) string {
	t.Helper()
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "node.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestClosedNodeCannotOverrideDutyProfileThroughSourceFields(t *testing.T) {
	for _, field := range []string{"profile", "authority"} {
		t.Run(field, func(t *testing.T) {
			plan := forwardingNodePlan(t)
			if field == "profile" {
				plan.StateProfile = "ardents-route-v3"
			} else {
				plan.StateProfileAuthority = plan.ClosedProfileAuthority
			}
			if _, err := readNodePlan(writeForwardingNodePlan(t, plan)); err == nil || !strings.Contains(err.Error(), "selected by its duty reservation") {
				t.Fatalf("Source-only field overrode Node duty profile: %v", err)
			}
		})
	}
}
