package enrollment

import (
	"context"
	"os"
	"syscall"
)

// Initial self-verification remains restricted to the selected Linux binary.
func infrastructurePlatform() error                                          { return ErrInput }
func verifyInfrastructureProcess(context.Context, os.FileInfo, []byte) error { return ErrInput }

func sameInfrastructureFile(a, b os.FileInfo) bool {
	if !sameFileState(a, b) {
		return false
	}
	x, ok := a.Sys().(*syscall.Stat_t)
	y, yes := b.Sys().(*syscall.Stat_t)
	return ok && yes && x.Ctimespec == y.Ctimespec && x.Uid == y.Uid && x.Gid == y.Gid && x.Nlink == 1 && y.Nlink == 1
}
