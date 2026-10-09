package installation

import (
	"bytes"
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
	if err != nil || !rootOwnedFile(info) || info.Mode().Perm() != 0600 || info.Size() != 0 {
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
	if err := errors.Join(pathErr, handleErr); err != nil || !sameObservedDirectory(lease.identity, pathInfo) ||
		!sameObservedDirectory(lease.identity, handleInfo) {
		return errors.Join(ErrNativeUnavailable, err)
	}
	pathWriter, pathErr := lease.root.Lstat("writer.lock")
	handleWriter, handleErr := lease.writer.Stat()
	if err := errors.Join(pathErr, handleErr); err != nil || !sameOwnedFile(pathWriter, handleWriter) ||
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

// installedRoot retains existing native root custody and read observations
// under one original writer lease. Recovery admits private record effects;
// inspection and staged-generation matching admit no effects or Release proof.
type installedRoot struct {
	lease *installationLease
	*installedFiles
}

func openInstalledRoot(ctx context.Context, directory string) (result *installedRoot, returnedErr error) {
	if !canonicalPath(directory) || directory == "/" {
		return nil, ErrInput
	}
	ancestors, err := rootDirectoryAncestors(filepath.Dir(directory))
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(directory)
	if err != nil || info == nil {
		return nil, errors.Join(ErrBinding, err)
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || native.Gid == 0 || info.Mode() != os.ModeDir|0750 {
		return nil, ErrBinding
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	lease := &installationLease{root: root, path: directory, identity: info}
	defer func() {
		if returnedErr != nil {
			returnedErr = errors.Join(returnedErr, lease.close())
		}
	}()
	lease.writer, err = root.OpenFile("writer.lock", os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	if err := lease.observe(); err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lease.writer.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, err
	}
	if err := lease.observe(); err != nil {
		return nil, err
	}
	ancestors[directory] = info
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &installedRoot{lease: lease, installedFiles: &installedFiles{directory: directory, gid: native.Gid, files: make(map[string]fileObservation), directories: ancestors, mutableDirectories: make(map[string]os.FileInfo)}}, nil
}

func (reader *installedRoot) observe(ctx context.Context) error {
	if reader == nil || reader.lease == nil || reader.installedFiles == nil {
		return ErrInput
	}
	if err := reader.lease.observe(); err != nil {
		return err
	}
	if err := reader.installedFiles.observe(ctx); err != nil {
		return err
	}
	return errors.Join(reader.lease.observe(), ctx.Err())
}

// Retained private records stay under the same Installation writer and original
// inode observations. Durability or visibility never supplies recovery authority.
func (reader *installedRoot) syncObserved(ctx context.Context, filename string) (returnedErr error) {
	observed, exists := reader.files[filename]
	if !exists {
		return ErrBinding
	}
	if _, err := reader.readObserved(ctx, filename, int64(len(observed.body))+1, observed.mode, observed.gid, len(observed.body) == 0); err != nil {
		return err
	}
	file, err := os.OpenFile(filename, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close(), ctx.Err()) }()
	info, err := file.Stat()
	if err != nil || !sameObservedFile(observed.identity, info) {
		return errors.Join(ErrBinding, err)
	}
	if err := file.Sync(); err != nil {
		return err
	}
	return syncDirectDirectory(filepath.Dir(filename))
}

func (reader *installedRoot) writePrivate(ctx context.Context, filename string, body []byte) (returnedErr error) {
	if existing, retained := reader.files[filename]; retained {
		if !bytes.Equal(existing.body, body) {
			return ErrBinding
		}
		return reader.syncObserved(ctx, filename)
	}
	if err := reader.lease.observe(); err != nil {
		return err
	}
	root, err := os.OpenRoot(filepath.Dir(filename))
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, root.Close()) }()
	info, err := root.Stat(".")
	if err != nil || !sameObservedDirectory(reader.directories[filepath.Dir(filename)], info) || !privateJournalDirectory(info) {
		return errors.Join(ErrBinding, err)
	}
	created, err := writeStagedFile(ctx, root, filepath.Base(filename), body, 0600, 0)
	if created.identity != nil {
		reader.files[filename] = created
	}
	return errors.Join(err, syncStagingRoot(root), ctx.Err())
}

func (reader *installedRoot) removeObserved(ctx context.Context, filename string) (returnedErr error) {
	if err := reader.syncObserved(ctx, filename); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := os.OpenRoot(filepath.Dir(filename))
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, root.Close(), ctx.Err()) }()
	info, err := root.Lstat(filepath.Base(filename))
	if err != nil || !sameObservedFile(reader.files[filename].identity, info) {
		return errors.Join(ErrBinding, err)
	}
	if err := root.Remove(filepath.Base(filename)); err != nil {
		return err
	}
	delete(reader.files, filename)
	return syncStagingRoot(root)
}
