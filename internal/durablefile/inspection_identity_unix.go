//go:build linux || darwin

package durablefile

import (
	"os"
	"syscall"
)

// InodeChangeTime returns the inode's change time (ctime) normalized to a
// comparable Timespec on the Unix family (Linux and macOS). The change time
// advances on content and metadata mutation even when the modification time is
// restored, so it is the anti-substitution signal of the durable-store
// inspection idiom. ok is false for a nil or non-Unix observation.
func InodeChangeTime(info os.FileInfo) (syscall.Timespec, bool) {
	if info == nil {
		return syscall.Timespec{}, false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return syscall.Timespec{}, false
	}
	ctime, _ := inodeTimes(native)
	return ctime, true
}

// InodeModificationTime returns the inode's modification time (mtime)
// normalized to a comparable Timespec on the Unix family (Linux and macOS).
// ok is false for a nil or non-Unix observation.
func InodeModificationTime(info os.FileInfo) (syscall.Timespec, bool) {
	if info == nil {
		return syscall.Timespec{}, false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return syscall.Timespec{}, false
	}
	_, mtime := inodeTimes(native)
	return mtime, true
}

// SameFileIdentity reports whether two observations describe the same unchanged
// inode: the same device/inode (os.SameFile), exact mode, size, owner, group,
// link count, change time and modification time. It is the shared full-identity
// comparison of the durable-store inspection idiom; a substituted inode, a
// hardlink change, an access change or a content/metadata mutation refuses.
// Callers keep their own regular-file, ownership and mode policy around it.
func SameFileIdentity(a, b os.FileInfo) bool {
	if a == nil || b == nil || !os.SameFile(a, b) || a.Mode() != b.Mode() || a.Size() != b.Size() {
		return false
	}
	x, xok := a.Sys().(*syscall.Stat_t)
	y, yok := b.Sys().(*syscall.Stat_t)
	if !xok || !yok || x.Uid != y.Uid || x.Gid != y.Gid || x.Nlink != y.Nlink {
		return false
	}
	xc, xm := inodeTimes(x)
	yc, ym := inodeTimes(y)
	return xc == yc && xm == ym
}
