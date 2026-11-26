package command

import (
	"context"
	"io"
)

// Run dispatches Source operations after the caller handles retained repair.
func Run(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	var operation string
	if len(args) > 0 {
		switch args[0] {
		case "inspect-latest", "prepare-trust", "prepare-state":
			operation = args[0]
		}
	}
	return run(ctx, operation, args, out, diagnostic)
}
