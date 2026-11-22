package main

import (
	"context"
	"fmt"
	"io"

	"github.com/dianabuilds/ardents-network/internal/node/provisioning"
	"github.com/dianabuilds/ardents-network/internal/release/infrastructure/signing"
)

func runInfrastructureCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) == 0 {
		return runInfrastructure(ctx, args, out)
	}
	var err error
	switch args[0] {
	case "provision":
		err = provisioning.Run(args[1:], out)
	case "sign-release":
		err = signing.Run(args[1:], out)
	default:
		return runInfrastructure(ctx, args, out)
	}
	if err != nil {
		fmt.Fprintln(diagnostic, err)
		return 2
	}
	return 0
}
