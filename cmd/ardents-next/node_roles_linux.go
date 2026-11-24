//go:build linux

package main

import (
	"context"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
	nodecommand "github.com/dianabuilds/ardents-network/internal/node/command"
	"github.com/dianabuilds/ardents-network/internal/route/role"
)

func runNodeRole(ctx context.Context, operation string, args []string, out, diagnostic io.Writer) int {
	switch operation {
	case "forwarding":
		return runNodeForwarding(ctx, args, out, diagnostic)
	case "issuer":
		return runNodeIssuer(ctx, args, out, diagnostic)
	case "descriptor":
		return runNodeDescriptor(ctx, args, out, diagnostic)
	case "introduction":
		return runNodeIntroduction(ctx, args, out, diagnostic)
	case "join":
		return runNodeJoin(ctx, args, out, diagnostic)
	default:
		return 2
	}
}

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
	raw, retained, code := nodecommand.ReadCancelableRolePlan(ctx, "node.descriptor", args, out)
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
	raw, retained, code := nodecommand.ReadCancelableRolePlan(ctx, "node.introduction", args, out)
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
	raw, retained, code := nodecommand.ReadCancelableRolePlan(ctx, "node.join", args, out)
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

type nodeProcess interface {
	node.InvocationRole
	FirstFailure() *node.FailureDiagnostic
}

func nodeStateConfig(network *node.Network, participationRoot string) (state.Config, error) {
	config, err := (*state.AuthorityDeclaration)(network).Config()
	if err != nil {
		return state.Config{}, err
	}
	config.LocalRoleStateRoot = participationRoot
	return config, nil
}

// Each selected role supplies its real constructor and exact typed plan.
// The command owns the original State and closes it after the Node's joined
// completion, even when the bounded wait has already reported unavailability.
func runNodeWithState[T nodeProcess](ctx context.Context, out, diagnostic io.Writer, operation string, config state.Config, nodeID [32]byte, waitMS uint32, listen func(role.Authority) (T, error)) int {
	owner, err := state.OpenRetained(config)
	if err != nil {
		_ = nodecommand.WriteEvent(out, operation, "refused", "state-unavailable", "", "", "")
		return 1
	}
	view, err := owner.CurrentRuntime()
	if err != nil {
		return nodecommand.StartupRefusal(out, operation, err, owner.Close())
	}
	duty, err := view.RetainDuty(nodeID, view.ObservedAt())
	if err != nil {
		return nodecommand.StartupRefusal(out, operation, err, owner.Close())
	}
	original, err := listen(role.Authority{Current: owner.CurrentRuntime, Duty: duty, Profile: view.ProfileBinding()})
	if err != nil {
		return nodecommand.StartupRefusal(out, operation, err, owner.Close())
	}
	result := node.SuperviseInvocation(ctx, original, time.Duration(waitMS)*time.Millisecond, func() error {
		return nodecommand.WriteEvent(out, operation, "listening", "original-duty", "", "", "")
	}, func() {
		nodecommand.ReportWaitExpired(diagnostic, operation, original.FirstFailure)
	}, owner.Close)
	return nodecommand.ReportCompletion(diagnostic, operation, result, original.FirstFailure)
}
