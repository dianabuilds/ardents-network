package main

import (
	"context"
	"errors"
	"syscall"
)

// Recovery retains only the original root-controlled public declaration. It
// loads no transport private key and starts no observer goroutine or runtime.
func acquireSourceRepairInputs(ctx context.Context, path string) (_ sourceInputs, plan sourceServePlan, result error) {
	if ctx == nil || ctx.Err() != nil {
		return sourceInputs{}, plan, errSourceInput
	}
	admission, deny := context.WithCancel(ctx)
	l := &sourceInputLifetime{ctx: admission, deny: deny, callback: make(chan struct{}), done: make(chan struct{})}
	inputs := sourceInputs{l}
	l.stop = context.AfterFunc(ctx, func() { l.refuse(ctx.Err()); close(l.callback) })
	defer func() {
		if result != nil {
			result = errors.Join(result, inputs.Close())
		}
	}()
	file, err := l.acquire(path, 16<<10, false)
	if err != nil {
		return sourceInputs{}, plan, err
	}
	native, ok := file.info.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || file.info.Mode().Perm()&0022 != 0 {
		return sourceInputs{}, plan, errSourceInput
	}
	plan, err = decodeSourceServePlan(file.raw)
	if err != nil || !validSourceServePlan(plan) {
		return sourceInputs{}, plan, errSourceInput
	}
	if err := inputs.Check(); err != nil {
		return sourceInputs{}, plan, err
	}
	return inputs, plan, nil
}
