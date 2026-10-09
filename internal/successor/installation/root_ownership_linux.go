package installation

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/directory"
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

func privateJournalDirectory(info os.FileInfo) bool {
	return rootDirectory(info) && info.Mode().Perm() == 0700
}

func syncDirectDirectory(directory string) (returnedErr error) {
	before, err := os.Lstat(directory)
	if err != nil || before == nil || !before.IsDir() || before.Mode().Perm()&0022 != 0 {
		return errors.Join(ErrNativeUnavailable, err)
	}
	owner, ok := before.Sys().(*syscall.Stat_t)
	if !ok || owner.Uid != 0 {
		return ErrNativeUnavailable
	}
	file, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return errors.Join(ErrNativeUnavailable, err)
	}
	if err := file.Sync(); err != nil {
		return err
	}
	after, err := os.Lstat(directory)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
		return errors.Join(ErrNativeUnavailable, err)
	}
	return nil
}

// Translate physical directory refusals without giving that Module admission.
func directoryObservationError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, directory.ErrInput):
		return errors.Join(ErrInput, err)
	case errors.Is(err, directory.ErrBinding):
		return errors.Join(ErrBinding, err)
	case errors.Is(err, directory.ErrNativeUnavailable):
		return errors.Join(ErrNativeUnavailable, err)
	default:
		return err
	}
}
