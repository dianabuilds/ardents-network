//go:build !windows

package nodelifecycle

import (
	"context"
	"errors"
	"os"
)

func writeEvent(ctx context.Context, output *os.File, raw []byte) (int, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0, errors.New("node lifecycle event deadline is missing")
	}
	if err := output.SetWriteDeadline(deadline); err != nil {
		return 0, err
	}
	return output.Write(raw)
}
