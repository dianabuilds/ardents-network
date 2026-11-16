//go:build linux

package main

import (
	"context"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/node"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
)

// The command owns genuine State; it closes that owner after original Node join.
func runNodeJoin(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	retained := len(args) == 2 && args[0] == "--retained-config"
	if retained {
		args = []string{"--config", args[1]}
	}
	if ctx == nil {
		return 2
	}
	if ctx.Err() != nil {
		if nodeJoinReport(out, "refused", "canceled", "", "") != nil {
			return 2
		}
		return 130
	}
	var plan struct {
		Network           *networkAuthorityPlan `json:"network"`
		NodeID            [32]byte              `json:"node_id"`
		ProcessProfile    string                `json:"process_profile"`
		ParticipationRoot string                `json:"participation_root"`
		SpendRoot         string                `json:"spend_root"`
		HostingRoot       string                `json:"hosting_root"`
		Certificate       string                `json:"certificate"`
		PrivateKey        string                `json:"private_key"`
		Work              hosting.Traffic       `json:"work"`
		Termination       hosting.Traffic       `json:"termination"`
		RetirementWaitMS  uint32                `json:"retirement_wait_ms"`
	}
	if admissionConfig(args, &plan) != nil || plan.Network == nil || plan.ProcessProfile != processresource.JoinProfile || plan.RetirementWaitMS == 0 || plan.RetirementWaitMS > 5000 {
		return 2
	}
	if !validAdmissionAuthority("", plan.Network, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot) || !independentRouteRoots(plan.Network.Root, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot) {
		return 2
	}
	config, err := networkStateConfig(plan.Network)
	if err != nil {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	config.LocalRoleStateRoot = plan.ParticipationRoot
	owner, err := state.OpenRetained(config)
	if err != nil {
		_ = nodeJoinReport(out, "refused", "state-unavailable", "", "")
		return 1
	}
	refuse := func(cause error) int {
		cleanup := ""
		if owner.Close() != nil {
			cleanup = "network"
		}
		if nodeJoinReport(out, "refused", "startup-unavailable", "", cleanup, encodeNodeFailureDiagnostic(node.StartupFailure(cause))) != nil {
			return 2
		}
		return 1
	}
	view, err := owner.CurrentRuntime()
	if err != nil {
		return refuse(err)
	}
	duty, err := view.RetainDuty(plan.NodeID, view.ObservedAt())
	if err != nil {
		return refuse(err)
	}
	listen := node.ListenJoin
	if retained {
		listen = node.ListenJoinRetained
	}
	original, err := listen(ctx, node.JoinConfig{
		Authority:         role.Authority{Current: owner.CurrentRuntime, Duty: duty, Profile: view.Profile().ProfileBinding},
		ParticipationRoot: plan.ParticipationRoot, SpendRoot: plan.SpendRoot, HostingRoot: plan.HostingRoot,
		Certificate: plan.Certificate, PrivateKey: plan.PrivateKey, Work: plan.Work, Termination: plan.Termination,
	})
	if err != nil {
		return refuse(err)
	}
	outputErr := nodeJoinReport(out, "listening", "original-duty", "", "")
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
		joined <- nodeCompletion{outcome: original.JoinedOutcome(), first: first, cleanup: cleanup, stateError: owner.Close()}
	}()
	timer := time.NewTimer(time.Duration(plan.RetirementWaitMS) * time.Millisecond)
	defer timer.Stop()
	result, timedOut := awaitNodeCompletion(joined, timer.C, func() {
		_ = nodeJoinReport(diagnostic, "retiring", "completion-unavailable", "", "")
	})
	if timedOut || result.outcome != "canceled" || result.stateError != nil {
		if result.stateError != nil {
			result.cleanup = "network"
		}
		_ = nodeJoinReport(diagnostic, "joined", "failed", result.first, result.cleanup, encodeNodeFailureDiagnostic(original.FirstFailure()))
		if outputErr != nil {
			return 2
		}
		return 1
	}
	if outputErr != nil {
		return 2
	}
	if nodeJoinReport(diagnostic, "joined", "canceled", "", "") != nil {
		return 2
	}
	return 130
}
