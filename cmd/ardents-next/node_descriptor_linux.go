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
func runNodeDescriptor(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if ctx == nil {
		return 2
	}
	if ctx.Err() != nil {
		if nodeDescriptorReport(out, "refused", "canceled", "", "") != nil {
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
		DescriptorRoot    string                `json:"descriptor_root"`
		Certificate       string                `json:"certificate"`
		PrivateKey        string                `json:"private_key"`
		Work              hosting.Traffic       `json:"work"`
		Termination       hosting.Traffic       `json:"termination"`
		RetirementWaitMS  uint32                `json:"retirement_wait_ms"`
	}
	if admissionConfig(args, &plan) != nil || plan.Network == nil || plan.ProcessProfile != processresource.DescriptorProfile || plan.RetirementWaitMS == 0 || plan.RetirementWaitMS > 5000 {
		return 2
	}
	if !validAdmissionAuthority("", plan.Network, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, plan.DescriptorRoot) || !independentRouteRoots(plan.Network.Root, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, plan.DescriptorRoot) {
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
	owner, err := state.Open(config)
	if err != nil {
		_ = nodeDescriptorReport(out, "refused", "state-unavailable", "", "")
		return 1
	}
	refuse := func() int {
		_ = owner.Close()
		if nodeDescriptorReport(out, "refused", "startup-unavailable", "", "") != nil {
			return 2
		}
		return 1
	}
	view, err := owner.CurrentRuntime()
	if err != nil {
		return refuse()
	}
	duty, err := view.RetainDuty(plan.NodeID, view.ObservedAt())
	if err != nil {
		return refuse()
	}
	original, err := node.ListenDescriptor(ctx, node.DescriptorConfig{
		Authority:         role.Authority{Current: owner.CurrentRuntime, Duty: duty, Profile: view.Profile().ProfileBinding},
		ParticipationRoot: plan.ParticipationRoot, SpendRoot: plan.SpendRoot, HostingRoot: plan.HostingRoot, DescriptorRoot: plan.DescriptorRoot,
		Certificate: plan.Certificate, PrivateKey: plan.PrivateKey, Work: plan.Work, Termination: plan.Termination,
	})
	if err != nil {
		return refuse()
	}
	outputErr := nodeDescriptorReport(out, "listening", "original-duty", "", "")
	if outputErr == nil {
		select {
		case <-ctx.Done():
		case <-original.Done():
		}
	}
	original.Stop()
	type completion struct {
		outcome, first, cleanup string
		stateError              error
	}
	joined := make(chan completion, 1)
	go func() {
		_ = original.Close()
		first, cleanup := original.JoinedFailureOwners()
		joined <- completion{original.JoinedOutcome(), first, cleanup, owner.Close()}
	}()
	timer := time.NewTimer(time.Duration(plan.RetirementWaitMS) * time.Millisecond)
	defer timer.Stop()
	var result completion
	timedOut := false
	select {
	case result = <-joined:
	case <-timer.C:
		timedOut = true
		_ = nodeDescriptorReport(diagnostic, "retiring", "completion-unavailable", "", "")
		// A bounded report never releases original roots/producer/TLS/State or
		// exits the command before actual physical completion.
		result = <-joined
	}
	if outputErr != nil {
		return 2
	}
	if timedOut || result.outcome != "canceled" || result.stateError != nil {
		if result.stateError != nil {
			result.cleanup = "network"
		}
		_ = nodeDescriptorReport(diagnostic, "joined", "failed", result.first, result.cleanup)
		return 1
	}
	if nodeDescriptorReport(diagnostic, "joined", "canceled", "", "") != nil {
		return 2
	}
	return 130
}
