package main

import (
	"context"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route/role"
)

const nodeForwardingOperation = "node.forwarding"

type nodeProcess interface {
	node.InvocationRole
	FirstFailure() *node.FailureDiagnostic
}

// Each selected role supplies its real constructor and exact typed plan.
// The command owns the original State and closes it after the Node's joined
// completion, even when the bounded wait has already reported unavailability.
func runNodeWithState[T nodeProcess](ctx context.Context, out, diagnostic io.Writer, operation string, config state.Config, nodeID [32]byte, waitMS uint32, listen func(role.Authority) (T, error)) int {
	owner, err := state.OpenRetained(config)
	if err != nil {
		_ = writeNodeEvent(out, operation, "refused", "state-unavailable", "", "", "")
		return 1
	}
	view, err := owner.CurrentRuntime()
	if err != nil {
		return nodeStartupRefusal(out, operation, err, owner.Close())
	}
	duty, err := view.RetainDuty(nodeID, view.ObservedAt())
	if err != nil {
		return nodeStartupRefusal(out, operation, err, owner.Close())
	}
	original, err := listen(role.Authority{Current: owner.CurrentRuntime, Duty: duty, Profile: view.ProfileBinding()})
	if err != nil {
		return nodeStartupRefusal(out, operation, err, owner.Close())
	}
	forwarding := operation == nodeForwardingOperation
	result := node.SuperviseInvocation(ctx, original, time.Duration(waitMS)*time.Millisecond, func() error {
		return writeNodeEvent(out, operation, "listening", "original-duty", "", "", "")
	}, func() {
		detail := ""
		if forwarding {
			detail = encodeNodeFailureDiagnostic(original.FirstFailure())
		}
		_ = writeNodeEvent(diagnostic, operation, "retiring", "completion-unavailable", "", "", detail)
	}, owner.Close)
	return reportNodeCompletion(diagnostic, operation, forwarding, result, original.FirstFailure)
}
