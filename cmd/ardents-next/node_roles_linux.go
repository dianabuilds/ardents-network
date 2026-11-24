//go:build linux

package main

import (
	"context"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/route/role"
)

// readNodeRolePlan owns the shared configuration flags and bounded file read.
// Role adapters decode their own declarations and open their original owners.
func readNodeRolePlan(args []string) (raw []byte, retained bool, err error) {
	if len(args) != 2 {
		return nil, false, errors.New("invalid Node role arguments")
	}
	switch args[0] {
	case "--config":
	case "--retained-config":
		retained = true
	default:
		return nil, false, errors.New("invalid Node role configuration flag")
	}
	raw, err = readBounded(args[1], 16<<10)
	return raw, retained, err
}

// Command composition retains State until the original Node has physically
// joined. A bounded failed wait cannot close it or report completed retirement.
func runNodeForwarding(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	raw, retained, err := readNodeRolePlan(args)
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
	listen := node.ListenForwarding
	if retained {
		listen = node.ListenForwardingRetained
	}
	return runNodeWithState(ctx, out, diagnostic, nodeForwardingOperation, config, plan.NodeID, plan.RetirementWaitMS, func(authority role.Authority) (*node.Forwarding, error) {
		return listen(ctx, node.ForwardingConfig{
			Authority:         authority,
			ParticipationRoot: plan.ParticipationRoot,
			SpendRoot:         plan.SpendRoot,
			HostingRoot:       plan.HostingRoot,
			Certificate:       plan.Certificate,
			PrivateKey:        plan.PrivateKey,
			Work:              plan.Work,
			Termination:       plan.Termination,
		})
	})
}

// State is owned here; Node joins its original dependents before State closes.
func runNodeIssuer(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if ctx == nil {
		return 2
	}
	raw, retained, err := readNodeRolePlan(args)
	if err != nil {
		return 2
	}
	plan, err := node.DecodeIssuerPlan(raw)
	if err != nil {
		return 2
	}
	network := (*networkAuthorityPlan)(plan.Network)
	if plan.Network == nil || plan.ProcessProfile != processresource.IssuerProfile || plan.RetirementWaitMS == 0 || plan.RetirementWaitMS > 5000 {
		return 2
	}
	prepared, err := decodeIssuanceResultPlan(plan.Issuance, "initialize-results")
	if err != nil || !validAdmissionAuthority("", network, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, prepared.Admission.Root, prepared.Keys.Root, prepared.Root) || !independentRouteRoots(network.Root, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, prepared.Admission.Root, prepared.Keys.Root, prepared.Root) {
		return 2
	}
	config, err := networkStateConfig(network)
	if err != nil {
		return 2
	}
	config.LocalRoleStateRoot = plan.ParticipationRoot
	listen := node.ListenIssuer
	if retained {
		listen = node.ListenIssuerRetained
	}
	return runNodeWithState(ctx, out, diagnostic, "node.issuer", config, plan.NodeID, plan.RetirementWaitMS, func(authority role.Authority) (*node.Issuer, error) {
		return listen(ctx, node.IssuerConfig{
			Authority:         authority,
			ParticipationRoot: plan.ParticipationRoot,
			SpendRoot:         plan.SpendRoot,
			HostingRoot:       plan.HostingRoot,
			Certificate:       plan.Certificate,
			PrivateKey:        plan.PrivateKey,
			Issuance: issuer.Plan{
				AdmissionRoot:    prepared.Admission.Root,
				AdmissionBinding: prepared.Admission.Binding,
				KeyRoot:          prepared.Keys.Root,
				KeyBinding:       prepared.Keys.Binding,
				ResultRoot:       prepared.Root,
			},
			Work:          plan.Work,
			BootstrapWork: plan.BootstrapWork,
			Termination:   plan.Termination,
		})
	})
}

// The command owns genuine State; it closes that owner after original Node join.
func runNodeDescriptor(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if ctx == nil {
		return 2
	}
	if ctx.Err() != nil {
		if writeNodeEvent(out, "node.descriptor", "refused", "canceled", "", "", "") != nil {
			return 2
		}
		return 130
	}
	raw, retained, err := readNodeRolePlan(args)
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

// The command owns genuine State; it closes that owner after original Node join.
func runNodeIntroduction(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if ctx == nil {
		return 2
	}
	if ctx.Err() != nil {
		if writeNodeEvent(out, "node.introduction", "refused", "canceled", "", "", "") != nil {
			return 2
		}
		return 130
	}
	raw, retained, err := readNodeRolePlan(args)
	if err != nil {
		return 2
	}
	plan, err := node.DecodeIntroductionPlan(raw)
	if err != nil {
		return 2
	}
	network := (*networkAuthorityPlan)(plan.Network)
	if plan.Network == nil || plan.ProcessProfile != processresource.IntroductionProfile || plan.RetirementWaitMS == 0 || plan.RetirementWaitMS > 5000 {
		return 2
	}
	if !validAdmissionAuthority("", network, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, plan.IntroductionRoot) || !independentRouteRoots(network.Root, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, plan.IntroductionRoot) {
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
	listen := node.ListenIntroduction
	if retained {
		listen = node.ListenIntroductionRetained
	}
	return runNodeWithState(ctx, out, diagnostic, "node.introduction", config, plan.NodeID, plan.RetirementWaitMS, func(authority role.Authority) (*node.Introduction, error) {
		return listen(ctx, node.IntroductionConfig{
			Authority:         authority,
			ParticipationRoot: plan.ParticipationRoot,
			SpendRoot:         plan.SpendRoot,
			HostingRoot:       plan.HostingRoot,
			IntroductionRoot:  plan.IntroductionRoot,
			Certificate:       plan.Certificate,
			PrivateKey:        plan.PrivateKey,
			Work:              plan.Work,
			Termination:       plan.Termination,
		})
	})
}

// The command owns genuine State; it closes that owner after original Node join.
func runNodeJoin(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if ctx == nil {
		return 2
	}
	if ctx.Err() != nil {
		if writeNodeEvent(out, "node.join", "refused", "canceled", "", "", "") != nil {
			return 2
		}
		return 130
	}
	raw, retained, err := readNodeRolePlan(args)
	if err != nil {
		return 2
	}
	plan, err := node.DecodeJoinPlan(raw)
	if err != nil {
		return 2
	}
	network := (*networkAuthorityPlan)(plan.Network)
	if plan.Network == nil || plan.ProcessProfile != processresource.JoinProfile || plan.RetirementWaitMS == 0 || plan.RetirementWaitMS > 5000 {
		return 2
	}
	if !validAdmissionAuthority("", network, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot) || !independentRouteRoots(network.Root, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot) {
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
	listen := node.ListenJoin
	if retained {
		listen = node.ListenJoinRetained
	}
	return runNodeWithState(ctx, out, diagnostic, "node.join", config, plan.NodeID, plan.RetirementWaitMS, func(authority role.Authority) (*node.Join, error) {
		return listen(ctx, node.JoinConfig{
			Authority:         authority,
			ParticipationRoot: plan.ParticipationRoot,
			SpendRoot:         plan.SpendRoot,
			HostingRoot:       plan.HostingRoot,
			Certificate:       plan.Certificate,
			PrivateKey:        plan.PrivateKey,
			Work:              plan.Work,
			Termination:       plan.Termination,
		})
	})
}
