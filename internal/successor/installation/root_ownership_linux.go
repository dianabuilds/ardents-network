package installation

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// These predicates admit trusted root-owned paths for the Installation lease,
// staging and independent inspection; they carry no request-file provenance.
func ownedRequestFile(info os.FileInfo) bool {
	if info == nil || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == 0 && native.Nlink == 1
}

func sameRequestFile(before, after os.FileInfo) bool {
	if !ownedRequestFile(before) || !ownedRequestFile(after) || !os.SameFile(before, after) ||
		before.Size() != after.Size() || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
		return false
	}
	left, leftOK := before.Sys().(*syscall.Stat_t)
	right, rightOK := after.Sys().(*syscall.Stat_t)
	return leftOK && rightOK && left.Ctim == right.Ctim
}

func rootDirectory(info os.FileInfo) bool {
	if info == nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == 0
}

func rootDirectoryAncestors(directory string) (map[string]os.FileInfo, error) {
	if !canonicalPath(directory) {
		return nil, ErrNativeUnavailable
	}
	observed := make(map[string]os.FileInfo)
	for {
		info, err := os.Lstat(directory)
		if err != nil || !rootDirectory(info) {
			return nil, errors.Join(ErrNativeUnavailable, err)
		}
		observed[directory] = info
		parent := filepath.Dir(directory)
		if parent == directory {
			return observed, nil
		}
		directory = parent
	}
}
