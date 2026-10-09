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
func rootOwnedFile(info os.FileInfo) bool {
	if info == nil || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == 0 && native.Nlink == 1
}

func sameOwnedFile(before, after os.FileInfo) bool {
	if !rootOwnedFile(before) || !rootOwnedFile(after) || !os.SameFile(before, after) ||
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

// fileObservation is detached file identity and bytes, shared by staged writes
// and independent installed reads. It owns no descriptor, lease or admission.
type fileObservation struct {
	identity os.FileInfo
	body     []byte
	gid      uint32
	mode     os.FileMode
}

// Independent reads retain ctime as well as access, size and modification time.
func sameObservedFile(before, after os.FileInfo) bool {
	if before == nil || after == nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return false
	}
	a, aOK := before.Sys().(*syscall.Stat_t)
	b, bOK := after.Sys().(*syscall.Stat_t)
	return aOK && bOK && a.Uid == b.Uid && a.Gid == b.Gid && a.Nlink == 1 && b.Nlink == 1 && a.Ctim == b.Ctim
}

func sameObservedDirectory(original, current os.FileInfo) bool {
	if original == nil || current == nil || !current.IsDir() || !os.SameFile(original, current) || original.Mode() != current.Mode() {
		return false
	}
	a, aOK := original.Sys().(*syscall.Stat_t)
	b, bOK := current.Sys().(*syscall.Stat_t)
	return aOK && bOK && a.Uid == 0 && b.Uid == 0 && a.Gid == b.Gid && current.Mode().Perm()&0022 == 0
}

// Staged-file matching checks the caller's expected final size and access;
// unlike independent read retention, it does not compare the original ctime.
func observedFileMatches(expected fileObservation, info os.FileInfo) bool {
	if expected.identity == nil || info == nil || !info.Mode().IsRegular() || !os.SameFile(expected.identity, info) ||
		info.Mode() != expected.mode || info.Size() != int64(len(expected.body)) || !info.ModTime().Equal(expected.identity.ModTime()) {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == expected.gid && native.Nlink == 1
}
