package command

import (
	"context"
	"io"
)

func run(ctx context.Context, operation string, args []string, out, diagnostic io.Writer) int {
	switch operation {
	case "inspect-latest":
		return runSourceInspection(ctx, args[1:], out, diagnostic)
	case "prepare-trust":
		return runSourcePreparation(ctx, args, out)
	case "prepare-state":
		return runSourceStatePreparation(ctx, args[1:], out, diagnostic)
	default:
		return runSource(ctx, args, out, diagnostic)
	}
}
