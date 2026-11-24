package command

import (
	"context"
	"io"
)

// Run dispatches the native Source command after the caller handles retained repair.
func Run(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "inspect-latest":
			return runSourceInspection(ctx, args[1:], out, diagnostic)
		case "prepare-trust":
			return runSourcePreparation(ctx, args, out)
		case "prepare-state":
			return runSourceStatePreparation(ctx, args[1:], out, diagnostic)
		}
	}
	return runSource(ctx, args, out, diagnostic)
}
