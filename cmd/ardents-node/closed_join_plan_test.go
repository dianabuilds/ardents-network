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

func dataJoinNodePlan(t *testing.T) nodePlan {
	t.Helper()
	plan := forwardingNodePlan(t)
	plan.ClosedForwarding = nil
	plan.ClosedDataJoin = &closedDataJoinPlan{AdmissionRoot: t.TempDir(), ConnectionLimit: 2, DrainTimeoutMS: 2000}
	return plan
}
func TestNodePlanConnectsClosedDataJoinToStateOwnedRuntime(t *testing.T) {
	plan := dataJoinNodePlan(t)
	path := writeForwardingNodePlan(t, plan)
	runtime, err := readNodePlan(path)
	if err != nil {
		t.Fatal(err)
	}
	local := runtime.node.ClosedDataJoin
	if runtime.state.AcceptedProfile != route.ClosedRouteProfile || !bytes.Equal(runtime.state.ClosedProfileAuthority, bytes.Repeat([]byte{0x12}, 32)) ||
		local.AdmissionRoot != plan.ClosedDataJoin.AdmissionRoot || local.ConnectionLimit != 2 ||
		local.DrainTimeout != 2*time.Second || local.Certificate.PrivateKey == nil || runtime.node.Probe.ListenAddress != "" || runtime.node.ClosedIssuer.Root != "" || runtime.node.ClosedForwarding.Root != "" {
		t.Fatal("JOIN reservation changed or bypassed current State")
	}
	if err := runIssuerNode(t.Context(), path, new(bytes.Buffer)); err == nil {
		t.Fatal("issuer command accepted JOIN reservation")
	}
}
func TestNodePlanRefusesMixedOrUnpinnedClosedDataJoin(t *testing.T) {
	for name, change := range map[string]func(*nodePlan){
		"missing pin":      func(p *nodePlan) { p.ClosedProfileAuthority = "" },
		"foreign pin":      func(p *nodePlan) { p.ClosedProfileAuthority = strings.Repeat("ff", 32) },
		"issuer":           func(p *nodePlan) { p.ClosedIssuer = &closedIssuerPlan{} },
		"resolution":       func(p *nodePlan) { p.ClosedResolution = &closedResolutionPlan{} },
		"forwarding":       func(p *nodePlan) { p.ClosedForwarding = &closedForwardingPlan{} },
		"rendezvous":       func(p *nodePlan) { p.Rendezvous = &rendezvousPlan{} },
		"introduction":     func(p *nodePlan) { p.Introduction = &introductionPlan{} },
		"resource profile": func(p *nodePlan) { p.NodeResourceProfile = "ardents-rendezvous-dedicated-host-v1" },
		"pin without duty": func(p *nodePlan) { p.ClosedDataJoin = nil },
	} {
		t.Run(name, func(t *testing.T) {
			plan := dataJoinNodePlan(t)
			change(&plan)
			if _, err := readNodePlan(writeForwardingNodePlan(t, plan)); err == nil {
				t.Fatal("mixed or unpinned JOIN accepted")
			}
		})
	}
}
func TestClosedDataJoinPlanCannotSupplyRemoteAuthority(t *testing.T) {
	plan := dataJoinNodePlan(t)
	for _, name := range []string{"endpoint", "peer", "assignment", "profile", "receiver_node_id", "verify", "storage_success"} {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			var local map[string]any
			if err := json.Unmarshal(fields["closed_data_join"], &local); err != nil {
				t.Fatal(err)
			}
			local[name] = "caller-selected"
			fields["closed_data_join"], err = json.Marshal(local)
			if err != nil {
				t.Fatal(err)
			}
			raw, err = json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "node.json")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := readNodePlan(path); err == nil {
				t.Fatal("remote authority injected through local plan")
			}
		})
	}
}
