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

func resolutionNodePlan(t *testing.T) nodePlan {
	t.Helper()
	plan := forwardingNodePlan(t)
	plan.ClosedForwarding = nil
	plan.ClosedResolution = &closedResolutionPlan{Root: t.TempDir(), AdmissionRoot: t.TempDir(), ConnectionLimit: 2, DrainTimeoutMS: 2000}
	return plan
}
func TestNodePlanConnectsClosedResolutionToStateOwnedRuntime(t *testing.T) {
	plan := resolutionNodePlan(t)
	path := writeForwardingNodePlan(t, plan)
	runtime, err := readNodePlan(path)
	if err != nil {
		t.Fatal(err)
	}
	local := runtime.node.ClosedResolution
	if runtime.state.AcceptedProfile != route.ClosedRouteProfile || !bytes.Equal(runtime.state.ClosedProfileAuthority, bytes.Repeat([]byte{0x12}, 32)) ||
		local.Root != plan.ClosedResolution.Root || local.AdmissionRoot != plan.ClosedResolution.AdmissionRoot || local.ConnectionLimit != 2 ||
		local.DrainTimeout != 2*time.Second || local.Certificate.PrivateKey == nil || runtime.node.Probe.ListenAddress != "" || runtime.node.ClosedIssuer.Root != "" || runtime.node.ClosedForwarding.Root != "" {
		t.Fatal("resolution reservation changed or bypassed current State")
	}
	if err := runIssuerNode(t.Context(), path, new(bytes.Buffer)); err == nil {
		t.Fatal("issuer command accepted resolution reservation")
	}
}
func TestNodePlanRefusesMixedOrUnpinnedClosedResolution(t *testing.T) {
	for name, change := range map[string]func(*nodePlan){
		"missing pin":      func(p *nodePlan) { p.ClosedProfileAuthority = "" },
		"foreign pin":      func(p *nodePlan) { p.ClosedProfileAuthority = strings.Repeat("ff", 32) },
		"issuer":           func(p *nodePlan) { p.ClosedIssuer = &closedIssuerPlan{} },
		"forwarding":       func(p *nodePlan) { p.ClosedForwarding = &closedForwardingPlan{} },
		"rendezvous":       func(p *nodePlan) { p.Rendezvous = &rendezvousPlan{} },
		"introduction":     func(p *nodePlan) { p.Introduction = &introductionPlan{} },
		"resource profile": func(p *nodePlan) { p.NodeResourceProfile = "ardents-rendezvous-dedicated-host-v1" },
		"pin without duty": func(p *nodePlan) { p.ClosedResolution = nil },
	} {
		t.Run(name, func(t *testing.T) {
			plan := resolutionNodePlan(t)
			change(&plan)
			if _, err := readNodePlan(writeForwardingNodePlan(t, plan)); err == nil {
				t.Fatal("mixed or unpinned resolution accepted")
			}
		})
	}
}
func TestClosedResolutionPlanCannotSupplyRemoteAuthority(t *testing.T) {
	plan := resolutionNodePlan(t)
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
			if err := json.Unmarshal(fields["closed_resolution"], &local); err != nil {
				t.Fatal(err)
			}
			local[name] = "caller-selected"
			fields["closed_resolution"], err = json.Marshal(local)
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
