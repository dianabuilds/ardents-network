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

func registrationNodePlan(t *testing.T) nodePlan {
	t.Helper()
	plan := forwardingNodePlan(t)
	plan.ClosedForwarding = nil
	plan.ClosedIntroduction = &closedIntroductionPlan{AdmissionRoot: t.TempDir(), ConnectionLimit: 2, DrainTimeoutMS: 2000}
	return plan
}
func TestNodePlanConnectsClosedIntroductionToStateOwnedRuntime(t *testing.T) {
	plan := registrationNodePlan(t)
	path := writeForwardingNodePlan(t, plan)
	runtime, err := readNodePlan(path)
	if err != nil {
		t.Fatal(err)
	}
	local := runtime.node.ClosedIntroduction
	if runtime.state.AcceptedProfile != route.ClosedRouteProfile || !bytes.Equal(runtime.state.ClosedProfileAuthority, bytes.Repeat([]byte{0x12}, 32)) ||
		local.AdmissionRoot != plan.ClosedIntroduction.AdmissionRoot || local.ConnectionLimit != 2 ||
		local.DrainTimeout != 2*time.Second || local.Certificate.PrivateKey == nil || runtime.node.Probe.ListenAddress != "" || runtime.node.ClosedIssuer.Root != "" || runtime.node.ClosedForwarding.Root != "" {
		t.Fatal("registration reservation changed or bypassed current State")
	}
	if err := runIssuerNode(t.Context(), path, new(bytes.Buffer)); err == nil {
		t.Fatal("issuer command accepted registration reservation")
	}
}
func TestNodePlanRefusesMixedOrUnpinnedClosedIntroduction(t *testing.T) {
	for name, change := range map[string]func(*nodePlan){
		"missing pin":      func(p *nodePlan) { p.ClosedProfileAuthority = "" },
		"foreign pin":      func(p *nodePlan) { p.ClosedProfileAuthority = strings.Repeat("ff", 32) },
		"issuer":           func(p *nodePlan) { p.ClosedIssuer = &closedIssuerPlan{} },
		"resolution":       func(p *nodePlan) { p.ClosedResolution = &closedResolutionPlan{} },
		"forwarding":       func(p *nodePlan) { p.ClosedForwarding = &closedForwardingPlan{} },
		"rendezvous":       func(p *nodePlan) { p.Rendezvous = &rendezvousPlan{} },
		"introduction":     func(p *nodePlan) { p.Introduction = &introductionPlan{} },
		"resource profile": func(p *nodePlan) { p.NodeResourceProfile = "ardents-rendezvous-dedicated-host-v1" },
		"pin without duty": func(p *nodePlan) { p.ClosedIntroduction = nil },
	} {
		t.Run(name, func(t *testing.T) {
			plan := registrationNodePlan(t)
			change(&plan)
			if _, err := readNodePlan(writeForwardingNodePlan(t, plan)); err == nil {
				t.Fatal("mixed or unpinned registration accepted")
			}
		})
	}
}
func TestClosedIntroductionPlanCannotSupplyRemoteAuthority(t *testing.T) {
	plan := registrationNodePlan(t)
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
			if err := json.Unmarshal(fields["closed_introduction"], &local); err != nil {
				t.Fatal(err)
			}
			local[name] = "caller-selected"
			fields["closed_introduction"], err = json.Marshal(local)
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
