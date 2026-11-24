package node

import (
	"context"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/role"
)

type receivingRolePlan interface {
	ValidInvocation() bool
	InvocationTarget() (*Network, string, [32]byte, uint32)
	Start(context.Context, role.Authority, bool) (RuntimeInvocation, error)
}

// RunRole owns the selected role and retained Network State lifetime.
func RunRole(ctx context.Context, operation string, args []string, out, diagnostic io.Writer) int {
	name := "node." + operation
	if !RuntimeAvailable {
		if err := WriteEvent(out, name, "refused", "unsupported-platform", "", "", ""); err != nil {
			return 2
		}
		return 1
	}
	switch operation {
	case "forwarding":
		return runNodeReceiving(ctx, ForwardingOperation, args, out, diagnostic, false, DecodeForwardingPlan)
	case "issuer":
		return runNodeIssuer(ctx, args, out, diagnostic)
	case "descriptor":
		return runNodeReceiving(ctx, name, args, out, diagnostic, true, DecodeDescriptorPlan)
	case "introduction":
		return runNodeReceiving(ctx, name, args, out, diagnostic, true, DecodeIntroductionPlan)
	case "join":
		return runNodeReceiving(ctx, name, args, out, diagnostic, true, DecodeJoinPlan)
	default:
		return 2
	}
}

func runNodeReceiving[P receivingRolePlan](ctx context.Context, operation string, args []string, out, diagnostic io.Writer, cancelableRead bool, decode func([]byte) (P, error)) int {
	var raw []byte
	var retained bool
	if cancelableRead {
		var code int
		raw, retained, code = ReadCancelableRolePlan(ctx, operation, args, out)
		if code != 0 {
			return code
		}
	} else {
		var err error
		raw, retained, err = ReadRolePlan(args)
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
	return runNodeWithState(ctx, out, diagnostic, operation, config, id, waitMS, func(authority role.Authority) (RuntimeInvocation, error) {
		return plan.Start(ctx, authority, retained)
	})
}

func runNodeIssuer(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if ctx == nil {
		return 2
	}
	raw, retained, err := ReadRolePlan(args)
	if err != nil {
		return 2
	}
	plan, err := DecodeIssuerPlan(raw)
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
	return runNodeWithState(ctx, out, diagnostic, "node.issuer", config, plan.NodeID, plan.RetirementWaitMS, func(authority role.Authority) (RuntimeInvocation, error) {
		return plan.Start(ctx, authority, retained, issuer.Plan{
			AdmissionRoot:    prepared.AdmissionRoot,
			AdmissionBinding: prepared.AdmissionBinding,
			KeyRoot:          prepared.KeyRoot,
			KeyBinding:       prepared.KeyBinding,
			ResultRoot:       prepared.ResultRoot,
		})
	})
}

func nodeStateConfig(network *Network, participationRoot string) (state.Config, error) {
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
func runNodeWithState(ctx context.Context, out, diagnostic io.Writer, operation string, config state.Config, nodeID [32]byte, waitMS uint32, listen func(role.Authority) (RuntimeInvocation, error)) int {
	owner, err := state.OpenRetained(config)
	if err != nil {
		if err := WriteEvent(out, operation, "refused", "state-unavailable", "", "", ""); err != nil {
			return 2
		}
		return 1
	}
	view, err := owner.CurrentRuntime()
	if err != nil {
		return StartupRefusal(out, operation, err, owner.Close())
	}
	duty, err := view.RetainDuty(nodeID, view.ObservedAt())
	if err != nil {
		return StartupRefusal(out, operation, err, owner.Close())
	}
	original, err := listen(role.Authority{Current: owner.CurrentRuntime, Duty: duty, Profile: view.ProfileBinding()})
	if err != nil {
		return StartupRefusal(out, operation, err, owner.Close())
	}
	result := SuperviseInvocation(ctx, original, time.Duration(waitMS)*time.Millisecond, func() error {
		return WriteEvent(out, operation, "listening", "original-duty", "", "", "")
	}, func() {
		ReportWaitExpired(diagnostic, operation, original.FirstFailure)
	}, owner.Close)
	return ReportCompletion(diagnostic, operation, result, original.FirstFailure)
}
