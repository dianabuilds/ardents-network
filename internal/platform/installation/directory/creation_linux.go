package directory

import (
	"os"
	"syscall"
)

func Matches(info os.FileInfo, uid, gid uint32) bool {
	if info == nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && uid != 0 && gid != 0 && native.Uid == uid && native.Gid == gid
}
