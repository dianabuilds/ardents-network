//go:build linux

package infrastructure

import (
	"context"
	"strconv"
)

// PrepareRuntimeCommand validates a native artifact request and produces its
// bounded public bytes with the original input and output custody.
func PrepareRuntimeCommand(ctx context.Context, args []string) ([]byte, error) {
	if len(args) < 2 {
		return nil, ErrInput
	}
	operation, role, values := args[0], args[1], args[2:]
	switch operation {
	case "prepare-runtime-descriptor":
		if len(values) != 5 {
			return nil, ErrInput
		}
		version, err := strconv.ParseInt(values[1], 10, 64)
		if err != nil || strconv.FormatInt(version, 10) != values[1] {
			return nil, ErrInput
		}
		return PrepareRuntimeDescriptorFile(ctx, role, values[0], version, values[2], values[3], values[4], nil)
	case "prepare-runtime-inventory":
		if len(values) != 2 {
			return nil, ErrInput
		}
		return PrepareRuntimeInventoryFile(ctx, role, values[0], values[1], nil)
	default:
		return nil, ErrInput
	}
}
