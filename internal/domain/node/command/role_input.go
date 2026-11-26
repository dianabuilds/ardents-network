package command

import (
	"context"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// readRolePlan owns the shared configuration flags and bounded file read.
// Role adapters decode their own declarations and open their original owners.
func readRolePlan(args []string) (raw []byte, retained bool, err error) {
	if len(args) != 2 {
		return nil, false, errors.New("invalid Node role arguments")
	}
	switch args[0] {
	case "--config":
	case "--retained-config":
		retained = true
	default:
		return nil, false, errors.New("invalid Node role configuration flag")
	}
	raw, err = artifacts.ReadBoundedFile(args[1], 16<<10)
	return raw, retained, err
}

// readCancelableRolePlan preserves the early cancellation receipt shared
// by Descriptor, Introduction, and JOIN, before touching the plan path.
func readCancelableRolePlan(ctx context.Context, operation string, args []string, out io.Writer) ([]byte, bool, int) {
	if ctx == nil {
		return nil, false, 2
	}
	if ctx.Err() != nil {
		if err := writeEvent(out, operation, "refused", "canceled", "", "", ""); err != nil {
			return nil, false, 2
		}
		return nil, false, 130
	}
	raw, retained, err := readRolePlan(args)
	if err != nil {
		return nil, false, 2
	}
	return raw, retained, 0
}
