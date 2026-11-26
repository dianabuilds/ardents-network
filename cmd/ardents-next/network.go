package main

import (
	"context"
	"io"

	statecommand "github.com/dianabuilds/ardents-network/internal/domain/network/state/command"
	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning"
)

func runNetwork(ctx context.Context, args []string, out io.Writer) int {
	if len(args) == 0 {
		return 2
	}
	switch args[0] {
	case "inspect-closed-profile":
		return runClosedProfileInspection(ctx, args[1:], out)
	case "accept-epoch", "accept-profile", "inspect":
		return statecommand.Run(ctx, args, out)
	default:
		return provisioning.RunNetworkArtifacts(ctx, args, out)
	}
}
