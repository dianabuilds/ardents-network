package main

import (
	"context"
	"io"
)

func runExecution(ctx context.Context, args []string, out io.Writer) int {
	if len(args) != 3 {
		return 2
	}
	switch args[0] {
	case "prepare-permission":
		return runAdmissionLocal(ctx, "execution-holder", args[1:], out, io.Discard)
	case "route-holder":
		return runAdmissionLocal(ctx, "execution-holder-live", args[1:], out, io.Discard)
	default:
		return 2
	}
}
