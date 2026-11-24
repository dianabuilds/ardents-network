package launch

import (
	"context"
	"io"

	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
)

// Run emits the fixed template or enters the native platform operation.
func Run(ctx context.Context, args []string, out io.Writer) int {
	if ctx != nil && ctx.Err() == nil && len(args) == 1 && args[0] == "template" {
		if _, err := io.WriteString(out, systemd.PlatformUnitTemplate); err != nil {
			return 1
		}
		return 0
	}
	return RunNative(ctx, args, out)
}
