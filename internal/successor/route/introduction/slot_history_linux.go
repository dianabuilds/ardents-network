//go:build linux

package introduction

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/spending"
)

const (
	slotHistoryName = "closed-introduction-slots"
	slotLeaseName   = ".ardents-introduction-lock"
)

// historyFiles owns the actual independently leased Linux root. Only the
// successful native creation/reopen path can publish a History using it.
type historyFiles struct {
	path                              string
	rootPath                          string
	root                              *os.File
	rootInfo, leaseInfo, snapshotInfo os.FileInfo
	snapshot                          *os.File
	lease                             *os.File
	watch                             int
	replace                           func(string, []byte) (*os.File, error)
}

func (files *historyFiles) commit(raw []byte) error {
	replace := files.replace
	if replace == nil {
		replace = replaceHistory
	}
	// Resolve mutations through the held original directory, never a pathname
	// that could now designate a replacement root. Readback pins our own lawful
	// atomic replacement only after all persistence steps have succeeded.
	path := filepath.Join(fmt.Sprintf("/proc/self/fd/%d", files.root.Fd()), slotHistoryName)
	snapshot, err := replace(path, raw)
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
	return errors.Join(snapshotErr, syscall.Flock(int(files.lease.Fd()), syscall.LOCK_UN), files.lease.Close(), syscall.Close(files.watch), files.root.Close())
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

func openHistory(root string, binding Binding, fact *spending.FreshRoot, syncRoot func(string) error) (_ *History, result error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || binding.NetworkID == [32]byte{} || binding.ProfileDigest == [32]byte{} || binding.ReceiverNodeID == [32]byte{} || binding.ReceiverDutyGeneration == 0 {
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
	anchoredRoot := fmt.Sprintf("/proc/self/fd/%d", directory.Fd())
	watch, err := syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		return nil, errors.Join(err, directory.Close())
	}
	if _, err := syscall.InotifyAddWatch(watch, anchoredRoot, syscall.IN_MOVE_SELF|syscall.IN_DELETE_SELF|syscall.IN_UNMOUNT|syscall.IN_ONLYDIR); err != nil {
		return nil, errors.Join(err, syscall.Close(watch), directory.Close())
	}
	lease, err := os.OpenFile(filepath.Join(anchoredRoot, slotLeaseName), flags, 0o600)
	if err != nil {
		return nil, errors.Join(err, syscall.Close(watch), directory.Close())
	}
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
		file, err := os.OpenFile(filepath.Join(anchoredRoot, slotHistoryName), os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
		if err != nil {
			return nil, err
		}
		raw := h.encode(h.entries, time.Time{})
		_, writeErr := file.Write(raw)
		if writeErr == nil {
			writeErr = file.Sync()
		}
		if err := errors.Join(writeErr, file.Close()); err != nil {
			return nil, err
		}
	}
	raw, err := readHistory(filepath.Join(anchoredRoot, slotHistoryName))
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
	if err := syncRoot(filepath.Join(anchoredRoot, slotHistoryName)); err != nil {
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

func sameHistoryFile(original, actual os.FileInfo) bool {
	if original == nil || actual == nil || !os.SameFile(original, actual) || original.Mode() != actual.Mode() || original.Size() != actual.Size() {
		return false
	}
	a, aok := original.Sys().(*syscall.Stat_t)
	b, bok := actual.Sys().(*syscall.Stat_t)
	return aok && bok && a.Ctim == b.Ctim && a.Uid == b.Uid && a.Gid == b.Gid && a.Nlink == b.Nlink
}

func (files *historyFiles) retainSnapshot(expected []byte) error {
	path := filepath.Join(fmt.Sprintf("/proc/self/fd/%d", files.root.Fd()), slotHistoryName)
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
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
	if err != nil || statErr != nil || !sameHistoryFile(info, after) || !bytes.Equal(raw, expected) {
		return errors.Join(errors.New("introduction snapshot readback differs"), err, statErr, file.Close())
	}
	named, err := os.Lstat(files.path)
	if err != nil || !sameHistoryFile(info, named) {
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
	var events [4096]byte
	n, err := syscall.Read(files.watch, events[:])
	if n > 0 || err != nil && !errors.Is(err, syscall.EAGAIN) {
		return errors.Join(errors.New("introduction original root movement or watch loss"), err)
	}
	root, err := os.Lstat(files.rootPath)
	heldRoot, heldErr := files.root.Stat()
	if err != nil || heldErr != nil || !sameHistoryFile(files.rootInfo, root) || !sameHistoryFile(files.rootInfo, heldRoot) {
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
	if err != nil || !sameHistoryFile(files.leaseInfo, held) || !sameHistoryFile(files.leaseInfo, retained) {
		return errors.Join(errors.New("introduction history lease lost"), err)
	}
	snapshot, err := os.Lstat(files.path)
	if err != nil || !sameHistoryFile(files.snapshotInfo, snapshot) {
		return errors.Join(errors.New("introduction original snapshot changed or lost"), err)
	}
	heldSnapshot, err := files.snapshot.Stat()
	if err != nil || !sameHistoryFile(files.snapshotInfo, heldSnapshot) {
		return errors.Join(errors.New("introduction original snapshot custody changed"), err)
	}
	raw, err := io.ReadAll(io.NewSectionReader(files.snapshot, 0, int64(slotHeaderSize+slotMaximum*40+1)))
	after, statErr := files.snapshot.Stat()
	if err != nil || statErr != nil || !sameHistoryFile(files.snapshotInfo, after) || !bytes.Equal(raw, expected) {
		return errors.Join(errors.New("introduction history changed or lost"), err, statErr)
	}
	return nil
}

func readHistory(path string) ([]byte, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
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

func syncHistory(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	if err := errors.Join(file.Sync(), file.Close()); err != nil {
		return err
	}
	return syncHistoryDirectory(filepath.Dir(path))
}

func syncHistoryDirectory(root string) error {
	directory, err := os.Open(root)
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}

func replaceHistory(path string, raw []byte) (*os.File, error) {
	file, err := os.CreateTemp(filepath.Dir(path), ".introduction-snapshot-")
	if err != nil {
		return nil, err
	}
	temporary := file.Name()
	// Interrupted/uncertain stages are conservative evidence. Strict reopen
	// refuses foreign/incomplete members; this owner never erases or repairs
	// such a footprint to manufacture a usable predecessor.
	_, writeErr := file.Write(raw)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	// Pin the actual staged inode before Rename. A later same-byte pathname
	// replacement cannot become our original committed snapshot.
	if writeErr != nil {
		return nil, errors.Join(writeErr, file.Close())
	}
	snapshot, err := os.OpenFile(temporary, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	written, writeStatErr := file.Stat()
	pinned, pinStatErr := snapshot.Stat()
	if writeStatErr != nil || pinStatErr != nil || !sameHistoryFile(written, pinned) {
		return nil, errors.Join(errors.New("introduction original stage substituted"), writeStatErr, pinStatErr, snapshot.Close(), file.Close())
	}
	if err := file.Close(); err != nil {
		return nil, errors.Join(err, snapshot.Close())
	}
	if err := os.Rename(temporary, path); err != nil {
		return nil, errors.Join(err, snapshot.Close())
	}
	if err := syncHistoryDirectory(filepath.Dir(path)); err != nil {
		return nil, errors.Join(err, snapshot.Close())
	}
	return snapshot, nil
}
