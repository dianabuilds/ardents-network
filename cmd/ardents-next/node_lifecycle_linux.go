package main

import (
	"context"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route/role"
)

type nodeCompletion struct {
	outcome        string
	stateError     error
	first, cleanup string
}

// A timeout reports unavailability but cannot make the original Node or State
// appear joined. The final result is retained until physical completion.
func awaitNodeCompletion(joined <-chan nodeCompletion, timeout <-chan time.Time, notify func()) (nodeCompletion, bool) {
	select {
	case result := <-joined:
		return result, false
	case <-timeout:
		notify()
		return <-joined, true
	}
}

const nodeForwardingOperation = "node.forwarding"

type nodeProcess interface {
	Stop()
	Done() <-chan struct{}
	Close() error
	JoinedOutcome() string
	JoinedFailureOwners() (string, string)
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
	outputErr := writeNodeEvent(out, operation, "listening", "original-duty", "", "", "")
	if outputErr == nil {
		select {
		case <-ctx.Done():
		case <-original.Done():
		}
	}
	original.Stop()
	joined := make(chan nodeCompletion, 1)
	go func() {
		_ = original.Close()
		first, cleanup := original.JoinedFailureOwners()
		joined <- nodeCompletion{outcome: original.JoinedOutcome(), first: first, cleanup: cleanup, stateError: owner.Close()}
	}()
	timer := time.NewTimer(time.Duration(waitMS) * time.Millisecond)
	defer timer.Stop()
	forwarding := operation == nodeForwardingOperation
	result, timedOut := awaitNodeCompletion(joined, timer.C, func() {
		detail := ""
		if forwarding {
			detail = encodeNodeFailureDiagnostic(original.FirstFailure())
		}
		_ = writeNodeEvent(diagnostic, operation, "retiring", "completion-unavailable", "", "", detail)
	})
	// Forwarding's established diagnostic contract suppresses a final event
	// on a timed-out but otherwise clean retirement. The other roles report
	// the retained wait failure even when the original close later succeeds.
	if result.outcome != "canceled" || result.stateError != nil || timedOut && !forwarding {
		if result.stateError != nil {
			result.cleanup = "network"
		}
		_ = writeNodeEvent(diagnostic, operation, "joined", "failed", result.first, result.cleanup,
			encodeNodeFailureDiagnostic(original.FirstFailure()))
		if outputErr != nil && (!forwarding || !timedOut) {
			return 2
		}
		return 1
	}
	if timedOut {
		return 1
	}
	if outputErr != nil {
		return 2
	}
	if writeNodeEvent(diagnostic, operation, "joined", "canceled", "", "", "") != nil {
		return 2
	}
	return 130
}

func nodeStartupRefusal(out io.Writer, operation string, cause, cleanup error) int {
	cleanupOwner := ""
	if cleanup != nil {
		cleanupOwner = "network"
	}
	if err := writeNodeEvent(out, operation, "refused", "startup-unavailable", "", cleanupOwner,
		encodeNodeFailureDiagnostic(node.StartupFailure(cause))); err != nil {
		return 2
	}
	return 1
}
