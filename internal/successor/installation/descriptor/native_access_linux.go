package descriptor

import (
	"os"
	"syscall"
)

func trustedInputDirectory(info os.FileInfo) bool {
	if info == nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == 0
}

func sameInputFile(before, after os.FileInfo) bool {
	if before == nil || after == nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() {
		return false
	}
	a, aok := before.Sys().(*syscall.Stat_t)
	b, bok := after.Sys().(*syscall.Stat_t)
	return aok && bok && a.Uid == b.Uid && a.Gid == b.Gid && a.Nlink == b.Nlink && a.Ctim == b.Ctim && a.Mtim == b.Mtim
}
