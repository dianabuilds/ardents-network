//go:build linux

package main

import (
	"context"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/route/role"
)

// The command owns genuine State; it closes that owner after original Node join.
func runNodeDescriptor(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	retained := len(args) == 2 && args[0] == "--retained-config"
	if retained {
		args = []string{"--config", args[1]}
	}
	if ctx == nil {
		return 2
	}
	if ctx.Err() != nil {
		if nodeDescriptorReport(out, "refused", "canceled", "", "") != nil {
			return 2
		}
		return 130
	}
	if len(args) != 2 || args[0] != "--config" {
		return 2
	}
	raw, err := readBounded(args[1], 16<<10)
	if err != nil {
		return 2
	}
	plan, err := node.DecodeDescriptorPlan(raw)
	if err != nil {
		return 2
	}
	network := (*networkAuthorityPlan)(plan.Network)
	if plan.Network == nil || plan.ProcessProfile != processresource.DescriptorProfile || plan.RetirementWaitMS == 0 || plan.RetirementWaitMS > 5000 {
		return 2
	}
	if !validAdmissionAuthority("", network, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, plan.DescriptorRoot) || !independentRouteRoots(network.Root, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, plan.DescriptorRoot) {
		return 2
	}
	config, err := networkStateConfig(network)
	if err != nil {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	config.LocalRoleStateRoot = plan.ParticipationRoot
	owner, err := state.OpenRetained(config)
	if err != nil {
		_ = nodeDescriptorReport(out, "refused", "state-unavailable", "", "")
		return 1
	}
	refuse := func(cause error) int {
		cleanup := ""
		if owner.Close() != nil {
			cleanup = "network"
		}
		if nodeDescriptorReport(out, "refused", "startup-unavailable", "", cleanup, encodeNodeFailureDiagnostic(node.StartupFailure(cause))) != nil {
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
	listen := node.ListenDescriptor
	if retained {
		listen = node.ListenDescriptorRetained
	}
	original, err := listen(ctx, node.DescriptorConfig{
		Authority:         role.Authority{Current: owner.CurrentRuntime, Duty: duty, Profile: view.Profile().ProfileBinding},
		ParticipationRoot: plan.ParticipationRoot, SpendRoot: plan.SpendRoot, HostingRoot: plan.HostingRoot, DescriptorRoot: plan.DescriptorRoot,
		Certificate: plan.Certificate, PrivateKey: plan.PrivateKey, Work: plan.Work, Termination: plan.Termination,
	})
	if err != nil {
		return refuse(err)
	}
	outputErr := nodeDescriptorReport(out, "listening", "original-duty", "", "")
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
		_ = nodeDescriptorReport(diagnostic, "retiring", "completion-unavailable", "", "")
	})
	if timedOut || result.outcome != "canceled" || result.stateError != nil {
		if result.stateError != nil {
			result.cleanup = "network"
		}
		_ = nodeDescriptorReport(diagnostic, "joined", "failed", result.first, result.cleanup, encodeNodeFailureDiagnostic(original.FirstFailure()))
		if outputErr != nil {
			return 2
		}
		return 1
	}
	if outputErr != nil {
		return 2
	}
	if nodeDescriptorReport(diagnostic, "joined", "canceled", "", "") != nil {
		return 2
	}
	return 130
}
