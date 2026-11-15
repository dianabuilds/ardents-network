package main

import (
	"context"
	"errors"
	"io"
	"os/signal"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/deployment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/introduction"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// A finite offline State producer, not an installed Node or Source runtime.
// Signed bytes are supplied publicly; State alone authenticates and persists
// them. No private key, participation, spend or Hosting root is opened.
func runIntroductionStatePreparation(args []string, out io.Writer) error {
	if len(args) != 3 || out == nil {
		return errors.New("usage: ardents-control introduction prepare-state PLAN BUNDLE SIGNED_PROFILE")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := prepareIntroductionState(ctx, args[0], args[1], args[2]); err != nil {
		return &introductionControlFailure{err}
	}
	_, err := io.WriteString(out, "{\"operation\":\"introduction.prepare-state\",\"state_accepted\":true,\"runtime_authority\":false}\n")
	return errors.Join(err, ctx.Err())
}

func prepareIntroductionState(ctx context.Context, planPath, bundlePath, profilePath string) error {
	return prepareNodeState(ctx, planPath, bundlePath, profilePath, introduction.ErrInput, introduction.ErrBinding, func(raw []byte) (state.Config, [32]byte, []string, error) {
		plan, err := deployment.DecodeIntroductionPlan(raw)
		if err != nil {
			return state.Config{}, [32]byte{}, nil, introduction.ErrInput
		}
		config, err := introductionPreparationStateConfig(plan)
		if err != nil {
			return state.Config{}, [32]byte{}, nil, err
		}
		return config, plan.NodeID, []string{config.Root, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, plan.IntroductionRoot}, nil
	})
}

func introductionPreparationStateConfig(plan deployment.IntroductionPlan) (state.Config, error) {
	if plan.Schema != "ardents-introduction-runtime-v1" || plan.ProcessProfile != "ardents-introduction-process-v1" || plan.NodeID == [32]byte{} {
		return state.Config{}, introduction.ErrInput
	}
	return nodePreparationStateConfig(plan.Network, []string{plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, plan.Certificate, plan.PrivateKey}, introduction.ErrInput, introduction.ErrBinding)
}
