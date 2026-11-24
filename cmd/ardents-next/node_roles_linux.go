//go:build linux

package main

import (
	"context"
	"io"

	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/node"
	nodecommand "github.com/dianabuilds/ardents-network/internal/node/command"
	"github.com/dianabuilds/ardents-network/internal/route/role"
)

// Command composition retains State until the original Node has physically
// joined. A bounded failed wait cannot close it or report completed retirement.
func runNodeForwarding(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	raw, retained, err := nodecommand.ReadRolePlan(args)
	if err != nil {
		return 2
	}
	plan, err := node.DecodeForwardingPlan(raw)
	if err != nil {
		return 2
	}
	if !plan.ValidInvocation() {
		return 2
	}
	config, err := nodeStateConfig(plan.Network, plan.ParticipationRoot)
	if err != nil {
		return 2
	}
	listen := node.ListenForwarding
	if retained {
		listen = node.ListenForwardingRetained
	}
	return runNodeWithState(ctx, out, diagnostic, nodecommand.ForwardingOperation, config, plan.NodeID, plan.RetirementWaitMS, func(authority role.Authority) (*node.Forwarding, error) {
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
	raw, retained, err := nodecommand.ReadRolePlan(args)
	if err != nil {
		return 2
	}
	plan, err := node.DecodeIssuerPlan(raw)
	if err != nil {
		return 2
	}
	prepared, err := issuance.DecodeResultSetupDeclaration(plan.Issuance)
	if err != nil || !plan.ValidInvocation(prepared.AdmissionRoot, prepared.KeyRoot, prepared.ResultRoot) {
		return 2
	}
	config, err := nodeStateConfig(plan.Network, plan.ParticipationRoot)
	if err != nil {
		return 2
	}
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
				AdmissionRoot:    prepared.AdmissionRoot,
				AdmissionBinding: prepared.AdmissionBinding,
				KeyRoot:          prepared.KeyRoot,
				KeyBinding:       prepared.KeyBinding,
				ResultRoot:       prepared.ResultRoot,
			},
			Work:          plan.Work,
			BootstrapWork: plan.BootstrapWork,
			Termination:   plan.Termination,
		})
	})
}

// The command owns genuine State; it closes that owner after original Node join.
func runNodeDescriptor(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	raw, retained, code := readCancelableNodeRolePlan(ctx, "node.descriptor", args, out)
	if code != 0 {
		return code
	}
	plan, err := node.DecodeDescriptorPlan(raw)
	if err != nil {
		return 2
	}
	if !plan.ValidInvocation() {
		return 2
	}
	config, err := nodeStateConfig(plan.Network, plan.ParticipationRoot)
	if err != nil {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
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
	raw, retained, code := readCancelableNodeRolePlan(ctx, "node.introduction", args, out)
	if code != 0 {
		return code
	}
	plan, err := node.DecodeIntroductionPlan(raw)
	if err != nil {
		return 2
	}
	if !plan.ValidInvocation() {
		return 2
	}
	config, err := nodeStateConfig(plan.Network, plan.ParticipationRoot)
	if err != nil {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
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
	raw, retained, code := readCancelableNodeRolePlan(ctx, "node.join", args, out)
	if code != 0 {
		return code
	}
	plan, err := node.DecodeJoinPlan(raw)
	if err != nil {
		return 2
	}
	if !plan.ValidInvocation() {
		return 2
	}
	config, err := nodeStateConfig(plan.Network, plan.ParticipationRoot)
	if err != nil {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
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
