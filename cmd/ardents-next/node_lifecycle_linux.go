package main

import (
	"context"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
	nodecommand "github.com/dianabuilds/ardents-network/internal/node/command"
	"github.com/dianabuilds/ardents-network/internal/route/role"
)

type nodeProcess interface {
	node.InvocationRole
	FirstFailure() *node.FailureDiagnostic
}

// readCancelableNodeRolePlan preserves the early cancellation receipt shared
// by Descriptor, Introduction, and JOIN, before touching the plan path.
func readCancelableNodeRolePlan(ctx context.Context, operation string, args []string, out io.Writer) ([]byte, bool, int) {
	if ctx == nil {
		return nil, false, 2
	}
	if ctx.Err() != nil {
		if err := nodecommand.WriteEvent(out, operation, "refused", "canceled", "", "", ""); err != nil {
			return nil, false, 2
		}
		return nil, false, 130
	}
	raw, retained, err := nodecommand.ReadRolePlan(args)
	if err != nil {
		return nil, false, 2
	}
	return raw, retained, 0
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
