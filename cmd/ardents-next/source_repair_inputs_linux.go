package main

import (
	"context"
	"errors"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/network/source/servingdeclaration"
)

// Recovery retains only the original root-controlled public declaration. It
// loads no transport private key and starts no observer goroutine or runtime.
func acquireSourceRepairInputs(ctx context.Context, path string) (_ sourceInputs, plan servingdeclaration.ServingDeclaration, result error) {
	if ctx == nil || ctx.Err() != nil {
		return sourceInputs{}, plan, errSourceInput
	}
	inputs := newSourceInputs(ctx, nil)
	defer func() {
		if result != nil {
			result = errors.Join(result, inputs.Close())
		}
	}()
	file, err := inputs.acquire(path, 16<<10, false)
	if err != nil {
		return sourceInputs{}, plan, err
	}
	native, ok := file.info.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || file.info.Mode().Perm()&0022 != 0 {
		return sourceInputs{}, plan, errSourceInput
	}
	plan, err = servingdeclaration.DecodeServingDeclaration(file.raw)
	if err != nil || !validSourceServePlan(plan) {
		return sourceInputs{}, plan, errSourceInput
	}
	if err := inputs.Check(); err != nil {
		return sourceInputs{}, plan, err
	}
	return inputs, plan, nil
}
