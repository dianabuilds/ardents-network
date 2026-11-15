package main

import (
	"context"
	"errors"
	"io"
	"os/signal"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/deployment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/forwarding"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// A finite offline State producer, not an installed Node or Source runtime.
// Signed bytes are supplied publicly; State alone authenticates and persists
// them. No private key, participation, spend or Hosting root is opened.
func runForwardingStatePreparation(args []string, out io.Writer) error {
	if len(args) != 3 || out == nil {
		return errors.New("usage: ardents-control forwarding prepare-state PLAN BUNDLE SIGNED_PROFILE")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := prepareForwardingState(ctx, args[0], args[1], args[2]); err != nil {
		return &forwardingControlFailure{err}
	}
	_, err := io.WriteString(out, "{\"operation\":\"forwarding.prepare-state\",\"state_accepted\":true,\"runtime_authority\":false}\n")
	return errors.Join(err, ctx.Err())
}

func prepareForwardingState(ctx context.Context, planPath, bundlePath, profilePath string) error {
	return prepareNodeState(ctx, planPath, bundlePath, profilePath, forwarding.ErrInput, forwarding.ErrBinding, func(raw []byte) (state.Config, [32]byte, []string, error) {
		plan, err := deployment.DecodeForwardingPlan(raw)
		if err != nil {
			return state.Config{}, [32]byte{}, nil, forwarding.ErrInput
		}
		config, err := forwardingPreparationStateConfig(plan)
		if err != nil {
			return state.Config{}, [32]byte{}, nil, err
		}
		return config, plan.NodeID, []string{config.Root, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot}, nil
	})
}

func forwardingPreparationStateConfig(plan deployment.ForwardingPlan) (state.Config, error) {
	if plan.Schema != "ardents-forwarding-runtime-v1" || plan.ProcessProfile != "ardents-forwarding-process-v1" || plan.NodeID == [32]byte{} {
		return state.Config{}, forwarding.ErrInput
	}
	return nodePreparationStateConfig(plan.Network, []string{plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, plan.Certificate, plan.PrivateKey}, forwarding.ErrInput, forwarding.ErrBinding)
}
