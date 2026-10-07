package installation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// A lease is a native mechanism, not installed ownership or Release authority.
// Only initial preparation creates this root. Reopen/recovery requires separate
// recorded ownership admission; mere root or lock presence is never adopted.
type installationLease struct {
	root     *os.Root
	writer   *os.File
	path     string
	identity os.FileInfo
	terminal error
}

func createInitialLease(ctx context.Context, directory string) (lease *installationLease, returnedErr error) {
	if ctx == nil || os.Geteuid() != 0 || !canonicalPath(directory) {
		return nil, ErrNativeUnavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := rootDirectoryAncestors(filepath.Dir(directory)); err != nil {
		return nil, err
	}
	if err := os.Mkdir(directory, 0700); err != nil {
		return nil, err
	}
	// Preserve any unrecorded birth on failure for explicit repair. No cleanup
	// or second initial attempt may silently accept the residue.
	if err := os.Chmod(directory, 0700); err != nil {
		return nil, err
	}
	identity, err := os.Lstat(directory)
	if err != nil || !privateJournalDirectory(identity) {
		return nil, errors.Join(ErrNativeUnavailable, err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	owned := &installationLease{root: root, path: directory, identity: identity}
	defer func() {
		if returnedErr != nil {
			owned.terminal = returnedErr
			returnedErr = owned.close()
		}
	}()
	writer, err := root.OpenFile("writer.lock", os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	owned.writer = writer
	info, err := writer.Stat()
	if err != nil || !ownedRequestFile(info) || info.Mode().Perm() != 0600 || info.Size() != 0 {
		return nil, errors.Join(ErrNativeUnavailable, err)
	}
	if err := syscall.Flock(int(writer.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, err
	}
	if err := writer.Sync(); err != nil {
		return nil, err
	}
	if err := syncDirectDirectory(directory); err != nil {
		return nil, err
	}
	if err := syncDirectDirectory(filepath.Dir(directory)); err != nil {
		return nil, err
	}
	if err := owned.observe(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return owned, nil
}

func (lease *installationLease) observe() error {
	if lease == nil || lease.root == nil || lease.writer == nil {
		return ErrNativeUnavailable
	}
	pathInfo, pathErr := os.Lstat(lease.path)
	handleInfo, handleErr := lease.root.Stat(".")
	if err := errors.Join(pathErr, handleErr); err != nil || !sameStagingDirectory(lease.identity, pathInfo) ||
		!sameStagingDirectory(lease.identity, handleInfo) {
		return errors.Join(ErrNativeUnavailable, err)
	}
	pathWriter, pathErr := lease.root.Lstat("writer.lock")
	handleWriter, handleErr := lease.writer.Stat()
	if err := errors.Join(pathErr, handleErr); err != nil || !sameRequestFile(pathWriter, handleWriter) ||
		handleWriter.Mode().Perm() != 0600 || handleWriter.Size() != 0 {
		return errors.Join(ErrNativeUnavailable, err)
	}
	return nil
}

func (lease *installationLease) close() error {
	if lease == nil {
		return nil
	}
	if lease.root != nil {
		lease.terminal = errors.Join(lease.terminal, lease.root.Close())
		lease.root = nil
	}
	// Keep the kernel lease until root-contained work has physically closed.
	if lease.writer != nil {
		lease.terminal = errors.Join(lease.terminal, lease.writer.Close())
		lease.writer = nil
	}
	return lease.terminal
}
