//go:build linux

package main

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/route/role"
)

// Command composition retains State until the original Node has physically
// joined. A bounded failed wait cannot close it or report completed retirement.
func runNodeForwarding(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	retained := len(args) == 2 && args[0] == "--retained-config"
	if retained {
		args = []string{"--config", args[1]}
	}
	if len(args) != 2 || args[0] != "--config" {
		return 2
	}
	raw, err := readBounded(args[1], 16<<10)
	if err != nil {
		return 2
	}
	plan, err := node.DecodeForwardingPlan(raw)
	if err != nil {
		return 2
	}
	network := (*networkAuthorityPlan)(plan.Network)
	if plan.Network == nil || plan.ProcessProfile != processresource.Profile || plan.RetirementWaitMS == 0 || plan.RetirementWaitMS > 5000 ||
		!validAdmissionAuthority("", network, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot) || !independentRouteRoots(network.Root, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot) {
		return 2
	}
	config, err := networkStateConfig(network)
	if err != nil {
		return 2
	}
	config.LocalRoleStateRoot = plan.ParticipationRoot
	owner, err := state.OpenRetained(config)
	if err != nil {
		_ = nodeReport(out, "refused", "state-unavailable")
		return 1
	}
	view, err := owner.CurrentRuntime()
	if err != nil {
		return nodeStartupRefusal(out, err, owner.Close())
	}
	duty, err := view.RetainDuty(plan.NodeID, view.ObservedAt())
	if err != nil {
		return nodeStartupRefusal(out, err, owner.Close())
	}
	listen := node.ListenForwarding
	if retained {
		listen = node.ListenForwardingRetained
	}
	original, err := listen(ctx, node.ForwardingConfig{Authority: role.Authority{Current: owner.CurrentRuntime, Duty: duty, Profile: view.Profile().ProfileBinding}, ParticipationRoot: plan.ParticipationRoot, SpendRoot: plan.SpendRoot, HostingRoot: plan.HostingRoot, Certificate: plan.Certificate, PrivateKey: plan.PrivateKey, Work: plan.Work, Termination: plan.Termination})
	if err != nil {
		return nodeStartupRefusal(out, err, owner.Close())
	}
	outputErr := nodeReport(out, "listening", "original-duty")
	if outputErr == nil {
		select {
		case <-ctx.Done():
		case <-original.Done():
		}
	}
	original.Stop()
	joined := make(chan nodeCompletion, 1)
	go func() {
		_ = original.Close()
		first, cleanup := original.JoinedFailureOwners()
		joined <- nodeCompletion{outcome: original.JoinedOutcome(), stateError: owner.Close(), first: first, cleanup: cleanup}
	}()
	timer := time.NewTimer(time.Duration(plan.RetirementWaitMS) * time.Millisecond)
	defer timer.Stop()
	result, timedOut := awaitNodeCompletion(joined, timer.C, func() {
		_ = nodeReport(diagnostic, "retiring", "completion-unavailable", encodeNodeFailureDiagnostic(original.FirstFailure()))
	})
	if result.outcome != "canceled" || result.stateError != nil {
		if result.stateError != nil {
			result.cleanup = "network"
		}
		_ = nodeJoinedFailure(diagnostic, result.first, result.cleanup, encodeNodeFailureDiagnostic(original.FirstFailure()))
		if outputErr != nil && !timedOut {
			return 2
		}
		return 1
	}
	if timedOut {
		return 1
	}
	if outputErr != nil {
		return 2
	}
	if nodeReport(diagnostic, "joined", "canceled") != nil {
		return 2
	}
	return 130
}

func nodeStartupRefusal(out io.Writer, cause, cleanup error) int {
	detail := encodeNodeFailureDiagnostic(node.StartupFailure(cause))
	var err error
	if cleanup == nil {
		err = nodeReport(out, "refused", "startup-unavailable", detail)
	} else {
		err = json.NewEncoder(out).Encode(struct {
			FirstFailure string `json:"first_failure,omitempty"`
			Operation    string `json:"operation"`
			Phase        string `json:"phase"`
			Outcome      string `json:"outcome"`
			Cleanup      string `json:"cleanup_owner"`
		}{detail, "node.forwarding", "refused", "startup-unavailable", "network"})
	}
	if err != nil {
		return 2
	}
	return 1
}

func nodeJoinedFailure(out io.Writer, first, cleanup string, details ...string) error {
	firstFailure := ""
	if len(details) == 1 {
		firstFailure = details[0]
	}
	return json.NewEncoder(out).Encode(struct {
		FirstFailure string `json:"first_failure,omitempty"`
		Operation    string `json:"operation"`
		Phase        string `json:"phase"`
		Outcome      string `json:"outcome"`
		First        string `json:"first_owner,omitempty"`
		Cleanup      string `json:"cleanup_owner,omitempty"`
	}{firstFailure, "node.forwarding", "joined", "failed", first, cleanup})
}
