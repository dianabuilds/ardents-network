//go:build linux || darwin

package introduction

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/durablefile"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
)

const (
	slotHistoryName = "closed-introduction-slots"
	slotLeaseName   = ".ardents-introduction-lock"
)

// historyFiles owns the actual independently leased Unix-family root. Only the
// successful native creation/reopen path can publish a History using it. Every
// mutation resolves through the held original directory descriptor (Openat /
// Renameat), never a pathname that could now designate a replacement root; the
// macOS /dev/fd spelling does not support directory traversal, so the shared
// directory-relative syscalls are the security-equivalent anchor on both
// platforms.
type historyFiles struct {
	path                              string
	rootPath                          string
	root                              *os.File
	rootInfo, leaseInfo, snapshotInfo os.FileInfo
	snapshot                          *os.File
	lease                             *os.File
	watch                             rootWatch
	replace                           func(int, []byte) (*os.File, error)
}

func (files *historyFiles) commit(raw []byte) error {
	replace := files.replace
	if replace == nil {
		replace = replaceHistory
	}
	// Resolve mutations through the held original directory, never a pathname
	// that could now designate a replacement root. Readback pins our own lawful
	// atomic replacement only after all persistence steps have succeeded.
	snapshot, err := replace(int(files.root.Fd()), raw)
	if err != nil {
		return err
	}
	if err := files.retainReplacement(snapshot, raw); err != nil {
		return err
	}
	info, err := files.root.Stat()
	if err != nil {
		return err
	}
	files.rootInfo = info
	return files.verify(raw)
}

func (files *historyFiles) close() error {
	var snapshotErr error
	if files.snapshot != nil {
		snapshotErr = files.snapshot.Close()
	}
	return errors.Join(snapshotErr, syscall.Flock(int(files.lease.Fd()), syscall.LOCK_UN), files.lease.Close(), files.watch.close(), files.root.Close())
}

// OpenHistory strictly reopens a complete retained root. Missing roots or
// members refuse; this operation never creates a marker or resets history.
func OpenHistory(root string, binding Binding) (*History, error) {
	return openHistory(root, binding, nil, syncHistory)
}

// InitializeHistory creates an empty independent root only from an opaque
// one-use fact of genuinely durable fresh Admission creation for this binding.
// Interrupted creation retains damage; reopen never reconstructs the fact.
func InitializeHistory(root string, binding Binding, fact *spending.FreshRoot) (*History, error) {
	if fact == nil {
		return nil, errors.New("introduction history initialization fact absent")
	}
	return openHistory(root, binding, fact, syncHistory)
}

func openHistory(root string, binding Binding, fact *spending.FreshRoot, syncRoot func(int) error) (_ *History, result error) {
	if !pathgrammar.Strict(root) || binding.NetworkID == [32]byte{} || binding.ProfileDigest == [32]byte{} || binding.ReceiverNodeID == [32]byte{} || binding.ReceiverDutyGeneration == 0 {
		return nil, errors.New("introduction history binding invalid")
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return nil, errors.Join(errors.New("introduction history private root unavailable"), err)
	}
	names, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var initialization *spending.RootInitialization
	if fact != nil {
		if len(names) != 0 {
			return nil, errors.New("introduction history initialization requires vacant root")
		}
		initialization, err = fact.Begin(spending.Binding{NetworkID: binding.NetworkID, ProfileDigest: binding.ProfileDigest, ReceiverNodeID: binding.ReceiverNodeID, ReceiverDutyGeneration: binding.ReceiverDutyGeneration})
		if err != nil {
			return nil, err
		}
		// Completion consumes the handoff even on failed I/O. The failed result
		// below cannot publish an owner; actual filesystem bytes remain retained.
		defer func() {
			if result != nil {
				result = errors.Join(result, initialization.Complete())
			}
		}()
	} else {
		if len(names) != 2 {
			return nil, errors.New("introduction history retained members missing or foreign")
		}
		for _, name := range names {
			if name.Name() != slotLeaseName && name.Name() != slotHistoryName {
				return nil, errors.New("introduction history retained member foreign")
			}
		}
	}
	flags := os.O_RDWR | syscall.O_NOFOLLOW | syscall.O_NONBLOCK
	if initialization != nil {
		if err := initialization.Check(); err != nil {
			return nil, err
		}
		flags |= os.O_CREATE | os.O_EXCL
	}
	directory, err := os.OpenFile(root, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	heldRoot, err := directory.Stat()
	if err != nil || !os.SameFile(info, heldRoot) || heldRoot.Mode().Perm()&0o077 != 0 {
		return nil, errors.Join(errors.New("introduction original root changed during open"), err, directory.Close())
	}
	rootfd := int(directory.Fd())
	watch, err := watchRoot(rootfd)
	if err != nil {
		return nil, errors.Join(err, directory.Close())
	}
	leaseFd, err := durablefile.Openat(rootfd, slotLeaseName, flags, 0o600)
	if err != nil {
		return nil, errors.Join(err, watch.close(), directory.Close())
	}
	lease := os.NewFile(uintptr(leaseFd), filepath.Join(root, slotLeaseName))
	files := &historyFiles{path: filepath.Join(root, slotHistoryName), rootPath: root, root: directory, lease: lease, watch: watch}
	h := &History{slotSnapshot: slotSnapshot{binding: binding, entries: make(map[[32]byte]time.Time)}, store: files}
	defer func() {
		if result != nil {
			result = errors.Join(result, h.Close())
		}
	}()
	info, err = lease.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() != 0 {
		return nil, errors.Join(errors.New("introduction history lease invalid"), err)
	}
	if err := syscall.Flock(int(lease.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, errors.New("introduction history already owned")
	}
	files.leaseInfo = info
	if initialization != nil {
		if err := initialization.Check(); err != nil {
			return nil, err
		}
		if err := lease.Sync(); err != nil {
			return nil, err
		}
		historyFd, err := durablefile.Openat(rootfd, slotHistoryName, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
		if err != nil {
			return nil, err
		}
		file := os.NewFile(uintptr(historyFd), filepath.Join(root, slotHistoryName))
		raw := h.encode(h.entries, time.Time{})
		_, writeErr := file.Write(raw)
		if writeErr == nil {
			writeErr = file.Sync()
		}
		if err := errors.Join(writeErr, file.Close()); err != nil {
			return nil, err
		}
	}
	raw, err := readHistory(rootfd, slotHistoryName)
	if err != nil {
		return nil, err
	}
	if err := h.decode(raw); err != nil {
		return nil, err
	}
	if initialization != nil {
		if err := initialization.Check(); err != nil {
			return nil, err
		}
	}
	if err := syncRoot(rootfd); err != nil {
		return nil, err
	}
	if initialization != nil {
		if err := initialization.Complete(); err != nil {
			return nil, err
		}
	}
	h.raw = bytes.Clone(raw)
	files.rootInfo, err = directory.Stat()
	if err != nil {
		return nil, err
	}
	if err := files.retainSnapshot(raw); err != nil {
		return nil, err
	}
	if err := files.verify(raw); err != nil {
		return nil, err
	}
	return h, nil
}

func (files *historyFiles) retainSnapshot(expected []byte) error {
	fd, err := durablefile.Openat(int(files.root.Fd()), slotHistoryName, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), files.path)
	return files.retainReplacement(file, expected)
}

func (files *historyFiles) retainReplacement(file *os.File, expected []byte) error {
	if file == nil {
		return errors.New("introduction original replacement snapshot absent")
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() != int64(len(expected)) {
		return errors.Join(errors.New("introduction original snapshot invalid"), err, file.Close())
	}
	raw, err := io.ReadAll(io.LimitReader(file, slotHeaderSize+slotMaximum*40+1))
	after, statErr := file.Stat()
	if err != nil || statErr != nil || !durablefile.SameFileIdentity(info, after) || !bytes.Equal(raw, expected) {
		return errors.Join(errors.New("introduction snapshot readback differs"), err, statErr, file.Close())
	}
	named, err := os.Lstat(files.path)
	if err != nil || !durablefile.SameFileIdentity(info, named) {
		return errors.Join(errors.New("introduction original replacement snapshot substituted"), err, file.Close())
	}
	if files.snapshot != nil {
		if err := files.snapshot.Close(); err != nil {
			return errors.Join(err, file.Close())
		}
	}
	files.snapshot, files.snapshotInfo = file, info
	return nil
}

func (files *historyFiles) verify(expected []byte) error {
	// Root-directory ctime also changes during our lawful snapshot replacement.
	// Its original kernel watch independently retains move/loss/restoration
	// evidence across that transaction; overflow or descriptor loss refuses.
	if err := files.watch.drain(); err != nil {
		return err
	}
	root, err := os.Lstat(files.rootPath)
	heldRoot, heldErr := files.root.Stat()
	if err != nil || heldErr != nil || !durablefile.SameFileIdentity(files.rootInfo, root) || !durablefile.SameFileIdentity(files.rootInfo, heldRoot) {
		return errors.Join(errors.New("introduction original root changed or lost"), err, heldErr)
	}
	names, err := os.ReadDir(filepath.Dir(files.path))
	if err != nil || len(names) != 2 {
		return errors.Join(errors.New("introduction history members changed or lost"), err)
	}
	for _, name := range names {
		if name.Name() != slotLeaseName && name.Name() != slotHistoryName {
			return errors.New("introduction history member foreign")
		}
	}
	retained, err := os.Lstat(filepath.Join(filepath.Dir(files.path), slotLeaseName))
	if err != nil {
		return err
	}
	held, err := files.lease.Stat()
	if err != nil || !durablefile.SameFileIdentity(files.leaseInfo, held) || !durablefile.SameFileIdentity(files.leaseInfo, retained) {
		return errors.Join(errors.New("introduction history lease lost"), err)
	}
	snapshot, err := os.Lstat(files.path)
	if err != nil || !durablefile.SameFileIdentity(files.snapshotInfo, snapshot) {
		return errors.Join(errors.New("introduction original snapshot changed or lost"), err)
	}
	heldSnapshot, err := files.snapshot.Stat()
	if err != nil || !durablefile.SameFileIdentity(files.snapshotInfo, heldSnapshot) {
		return errors.Join(errors.New("introduction original snapshot custody changed"), err)
	}
	raw, err := io.ReadAll(io.NewSectionReader(files.snapshot, 0, int64(slotHeaderSize+slotMaximum*40+1)))
	after, statErr := files.snapshot.Stat()
	if err != nil || statErr != nil || !durablefile.SameFileIdentity(files.snapshotInfo, after) || !bytes.Equal(raw, expected) {
		return errors.Join(errors.New("introduction history changed or lost"), err, statErr)
	}
	return nil
}

func readHistory(rootfd int, name string) ([]byte, error) {
	fd, err := durablefile.Openat(rootfd, name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), name)
	info, statErr := file.Stat()
	if statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return nil, errors.Join(errors.New("introduction snapshot invalid"), statErr, file.Close())
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, slotHeaderSize+slotMaximum*40+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return nil, err
	}
	if len(raw) > slotHeaderSize+slotMaximum*40 {
		return nil, errors.New("introduction history exceeds bound")
	}
	return raw, nil
}

func syncHistory(rootfd int) error {
	fd, err := durablefile.Openat(rootfd, slotHistoryName, os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), slotHistoryName)
	if err := errors.Join(file.Sync(), file.Close()); err != nil {
		return err
	}
	return syncHistoryDirectory(rootfd)
}

func syncHistoryDirectory(rootfd int) error {
	fd, err := durablefile.Openat(rootfd, ".", os.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	directory := os.NewFile(uintptr(fd), ".")
	return errors.Join(directory.Sync(), directory.Close())
}

func replaceHistory(rootfd int, raw []byte) (*os.File, error) {
	stageFd, stageName, err := createStage(rootfd)
	if err != nil {
		return nil, err
	}
	stage := os.NewFile(uintptr(stageFd), stageName)
	// Interrupted/uncertain stages are conservative evidence. Strict reopen
	// refuses foreign/incomplete members; this owner never erases or repairs
	// such a footprint to manufacture a usable predecessor.
	_, writeErr := stage.Write(raw)
	if writeErr == nil {
		writeErr = stage.Sync()
	}
	// Pin the actual staged inode before Rename. A later same-byte pathname
	// replacement cannot become our original committed snapshot.
	if writeErr != nil {
		return nil, errors.Join(writeErr, stage.Close())
	}
	pinFd, err := durablefile.Openat(rootfd, stageName, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.Join(err, stage.Close())
	}
	snapshot := os.NewFile(uintptr(pinFd), stageName)
	written, writeStatErr := stage.Stat()
	pinned, pinStatErr := snapshot.Stat()
	if writeStatErr != nil || pinStatErr != nil || !durablefile.SameFileIdentity(written, pinned) {
		return nil, errors.Join(errors.New("introduction original stage substituted"), writeStatErr, pinStatErr, snapshot.Close(), stage.Close())
	}
	if err := stage.Close(); err != nil {
		return nil, errors.Join(err, snapshot.Close())
	}
	if err := durablefile.Renameat(rootfd, stageName, rootfd, slotHistoryName); err != nil {
		return nil, errors.Join(err, snapshot.Close())
	}
	if err := syncHistoryDirectory(rootfd); err != nil {
		return nil, errors.Join(err, snapshot.Close())
	}
	return snapshot, nil
}

// createStage opens one uniquely named owner-only staging file inside the held
// directory, retrying only on a name collision. The interrupted stage is
// retained as conservative evidence; it is never erased here.
func createStage(rootfd int) (int, string, error) {
	for range 10000 {
		name := fmt.Sprintf(".introduction-snapshot-%d", rand.Uint64())
		fd, err := durablefile.Openat(rootfd, name, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
		if err == nil {
			return fd, name, nil
		}
		if !errors.Is(err, syscall.EEXIST) {
			return 0, "", err
		}
	}
	return 0, "", errors.New("introduction staging name exhausted")
}
