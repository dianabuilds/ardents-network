package hosting

import (
	"errors"
	"os"
	"syscall"
)

// ErrInitialFileChanged reports that an original period file no longer matches
// the file created by InitializeFiles. It grants no initialization authority.
var ErrInitialFileChanged = errors.New("hosting: original initial file changed")

// Promote transfers a file created by this initialization to the runtime
// principal. The caller remains responsible for admitting the original root,
// excluding other users and retaining any partial initialization failure.
func (births FileBirths) Promote(root *os.Root, name string, uid, gid uint32) (result os.FileInfo, returned error) {
	original := births.originals[name]
	if root == nil || (name != "budget.pin" && name != "budget.lock" && name != "budget.json") {
		return nil, ErrInitialFileChanged
	}
	before, err := root.Lstat(name)
	if err != nil || !sameInitialFile(original, before) || !initialFileAccess(before, 0, 0) {
		return nil, errors.Join(ErrInitialFileChanged, err)
	}
	file, err := root.OpenFile(name, os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer func() { returned = errors.Join(returned, file.Close()) }()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, errors.Join(ErrInitialFileChanged, err)
	}
	if err := file.Chown(int(uid), int(gid)); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	result, err = file.Stat()
	named, nameErr := root.Lstat(name)
	if err != nil || nameErr != nil || !os.SameFile(before, result) || !os.SameFile(result, named) || !initialFileAccess(result, uid, gid) || !initialFileAccess(named, uid, gid) {
		return nil, errors.Join(ErrInitialFileChanged, err, nameErr)
	}
	return result, nil
}

func sameInitialFile(before, after os.FileInfo) bool {
	if before == nil || after == nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() {
		return false
	}
	a, aok := before.Sys().(*syscall.Stat_t)
	b, bok := after.Sys().(*syscall.Stat_t)
	return aok && bok && a.Uid == b.Uid && a.Gid == b.Gid && a.Nlink == b.Nlink && a.Ctim == b.Ctim && a.Mtim == b.Mtim
}

func initialFileAccess(info os.FileInfo, uid, gid uint32) bool {
	if info == nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky|os.ModeSymlink) != 0 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == uid && native.Gid == gid && native.Nlink == 1
}
