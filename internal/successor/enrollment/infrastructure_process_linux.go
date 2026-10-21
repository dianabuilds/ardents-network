//go:build linux

package enrollment

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"syscall"
)

func infrastructurePlatform() error {
	if runtime.GOARCH != "amd64" {
		return ErrInput
	}
	return nil
}

func sameInfrastructureFile(a, b os.FileInfo) bool {
	if !sameFileState(a, b) {
		return false
	}
	x, ok := a.Sys().(*syscall.Stat_t)
	y, yes := b.Sys().(*syscall.Stat_t)
	return ok && yes && x.Ctim == y.Ctim && x.Uid == y.Uid && x.Gid == y.Gid && x.Nlink == 1 && y.Nlink == 1
}

func verifyInfrastructureProcess(ctx context.Context, enrolled os.FileInfo, raw []byte) (err error) {
	// /proc/self/exe identifies the actual mapped executable even if its
	// pathname was renamed or replaced after launch. A same-named copy fails.
	f, err := os.Open("/proc/self/exe")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close(), ctx.Err()) }()
	before, err := f.Stat()
	if err != nil {
		return err
	}
	if !sameFileState(enrolled, before) {
		return ErrBinding
	}
	actual, err := io.ReadAll(io.LimitReader(f, maximumFileLen+1))
	if err != nil {
		return err
	}
	after, err := f.Stat()
	if err != nil {
		return err
	}
	if !sameFileState(before, after) || !bytes.Equal(actual, raw) {
		return ErrBinding
	}
	return nil
}
