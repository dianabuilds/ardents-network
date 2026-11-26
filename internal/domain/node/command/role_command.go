package command

import (
	"context"
	"github.com/dianabuilds/ardents-network/internal/domain/node"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/measurement"
	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
)

// RunRole owns the selected role and retained Network State lifetime.
func RunRole(ctx context.Context, operation string, args []string, out, diagnostic io.Writer) int {
	name := "node." + operation
	if !node.RuntimeAvailable {
		if err := writeEvent(out, name, "refused", "unsupported-platform", "", "", ""); err != nil {
			return 2
		}
		return 1
	}
	switch operation {
	case "forwarding":
		return runNodeReceiving(ctx, forwardingOperation, args, out, diagnostic, false, node.DecodeForwardingPlan)
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

func runNodeReceiving[P node.ReceivingPlan](ctx context.Context, operation string, args []string, out, diagnostic io.Writer, cancelableRead bool, decode func([]byte) (P, error)) int {
	var raw []byte
	var retained bool
	if cancelableRead {
		var code int
		raw, retained, code = readCancelableRolePlan(ctx, operation, args, out)
		if code != 0 {
			return code
		}
	} else {
		var err error
		raw, retained, err = readRolePlan(args)
		if err != nil {
			return 2
		}
	}
	plan, err := decode(raw)
	if err != nil || !plan.ValidInvocation() {
		return 2
	}
	network, participationRoot, _, waitMS := plan.InvocationTarget()
	config, err := nodeStateConfig(network, participationRoot)
	if err != nil {
		return 2
	}
	if cancelableRead && ctx.Err() != nil {
		return 130
	}
	return runNodeWithState(ctx, out, diagnostic, operation, config, waitMS, func(owner *state.State) (node.RuntimeInvocation, error) {
		return node.StartReceivingRole(ctx, owner, plan, retained)
	})
}

func runNodeIssuer(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if ctx == nil {
		return 2
	}
	raw, retained, err := readRolePlan(args)
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
	return runNodeWithState(ctx, out, diagnostic, "node.issuer", config, plan.RetirementWaitMS, func(owner *state.State) (node.RuntimeInvocation, error) {
		return node.StartIssuingRole(ctx, owner, plan, retained, issuer.Plan{
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
func runNodeWithState(ctx context.Context, out, diagnostic io.Writer, operation string, config state.Config, waitMS uint32, listen func(*state.State) (node.RuntimeInvocation, error)) int {
	defer measurement.Begin(runtimeevent.NodeLifetime).End()

	owner, err := state.OpenRetained(config)
	if err != nil {
		if err := writeEvent(out, operation, "refused", "state-unavailable", "", "", ""); err != nil {
			return 2
		}
		return 1
	}
	original, err := listen(owner)
	if err != nil {
		return startupRefusal(out, operation, err, owner.Close())
	}
	result := node.SuperviseInvocation(ctx, original, time.Duration(waitMS)*time.Millisecond, func() error {
		return writeEvent(out, operation, "listening", "original-duty", "", "", "")
	}, func() {
		reportWaitExpired(diagnostic, operation, original.FirstFailure)
	}, owner.Close)
	return reportCompletion(diagnostic, operation, result, original.FirstFailure)
}
