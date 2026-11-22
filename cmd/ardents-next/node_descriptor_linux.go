//go:build linux

package main

import (
	"context"
	"io"

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
		if writeNodeEvent(out, "node.descriptor", "refused", "canceled", "", "", "") != nil {
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
	listen := node.ListenDescriptor
	if retained {
		listen = node.ListenDescriptorRetained
	}
	return runNodeWithState(ctx, out, diagnostic, "node.descriptor", config, plan.NodeID, plan.RetirementWaitMS, func(authority role.Authority) (*node.Descriptor, error) {
		return listen(ctx, node.DescriptorConfig{
			Authority:         authority,
			ParticipationRoot: plan.ParticipationRoot,
			SpendRoot:         plan.SpendRoot,
			HostingRoot:       plan.HostingRoot,
			DescriptorRoot:    plan.DescriptorRoot,
			Certificate:       plan.Certificate,
			PrivateKey:        plan.PrivateKey,
			Work:              plan.Work,
			Termination:       plan.Termination,
		})
	})
}
