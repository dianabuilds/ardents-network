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

type receivingRolePlan interface {
	ValidInvocation() bool
	InvocationTarget() (*node.Network, string, [32]byte, uint32)
	Start(context.Context, role.Authority, bool) (node.RuntimeInvocation, error)
}

func runNodeRole(ctx context.Context, operation string, args []string, out, diagnostic io.Writer) int {
	name := "node." + operation
	if !node.RuntimeAvailable {
		if err := nodecommand.WriteEvent(out, name, "refused", "unsupported-platform", "", "", ""); err != nil {
			return 2
		}
		return 1
	}
	switch operation {
	case "forwarding":
		return runNodeReceiving(ctx, nodecommand.ForwardingOperation, args, out, diagnostic, false, node.DecodeForwardingPlan)
	case "issuer":
		return runNodeIssuer(ctx, args, out, diagnostic)
	case "descriptor":
		return runNodeReceiving(ctx, name, args, out, diagnostic, true, node.DecodeDescriptorPlan)
	case "introduction":
		return runNodeReceiving(ctx, name, args, out, diagnostic, true, node.DecodeIntroductionPlan)
	case "join":
		return runNodeReceiving(ctx, name, args, out, diagnostic, true, node.DecodeJoinPlan)
	default:
		return 2
	}
}

func runNodeReceiving[P receivingRolePlan](ctx context.Context, operation string, args []string, out, diagnostic io.Writer, cancelableRead bool, decode func([]byte) (P, error)) int {
	var raw []byte
	var retained bool
	if cancelableRead {
		var code int
		raw, retained, code = nodecommand.ReadCancelableRolePlan(ctx, operation, args, out)
		if code != 0 {
			return code
		}
	} else {
		var err error
		raw, retained, err = nodecommand.ReadRolePlan(args)
		if err != nil {
			return 2
		}
	}
	plan, err := decode(raw)
	if err != nil || !plan.ValidInvocation() {
		return 2
	}
	network, participationRoot, id, waitMS := plan.InvocationTarget()
	config, err := nodeStateConfig(network, participationRoot)
	if err != nil {
		return 2
	}
	if cancelableRead && ctx.Err() != nil {
		return 130
	}
	return runNodeWithState(ctx, out, diagnostic, operation, config, id, waitMS, func(authority role.Authority) (node.RuntimeInvocation, error) {
		return plan.Start(ctx, authority, retained)
	})
}

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
	return runNodeWithState(ctx, out, diagnostic, "node.issuer", config, plan.NodeID, plan.RetirementWaitMS, func(authority role.Authority) (node.RuntimeInvocation, error) {
		return plan.Start(ctx, authority, retained, issuer.Plan{
			AdmissionRoot:    prepared.AdmissionRoot,
			AdmissionBinding: prepared.AdmissionBinding,
			KeyRoot:          prepared.KeyRoot,
			KeyBinding:       prepared.KeyBinding,
			ResultRoot:       prepared.ResultRoot,
		})
	})
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
func runNodeWithState(ctx context.Context, out, diagnostic io.Writer, operation string, config state.Config, nodeID [32]byte, waitMS uint32, listen func(role.Authority) (node.RuntimeInvocation, error)) int {
	owner, err := state.OpenRetained(config)
	if err != nil {
		if err := nodecommand.WriteEvent(out, operation, "refused", "state-unavailable", "", "", ""); err != nil {
			return 2
		}
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
