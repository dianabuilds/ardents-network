//go:build linux

package introduction

import (
	"bytes"
	"errors"
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
	path    string
	lease   *os.File
	replace func(string, []byte) error
}

func (files *historyFiles) commit(raw []byte) error {
	replace := files.replace
	if replace == nil {
		replace = replaceHistory
	}
	return replace(files.path, raw)
}

func (files *historyFiles) close() error {
	return errors.Join(syscall.Flock(int(files.lease.Fd()), syscall.LOCK_UN), files.lease.Close())
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
	lease, err := os.OpenFile(filepath.Join(root, slotLeaseName), flags, 0o600)
	if err != nil {
		return nil, err
	}
	files := &historyFiles{path: filepath.Join(root, slotHistoryName), lease: lease}
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
	if initialization != nil {
		if err := initialization.Check(); err != nil {
			return nil, err
		}
		if err := lease.Sync(); err != nil {
			return nil, err
		}
		file, err := os.OpenFile(files.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
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
	raw, err := readHistory(files.path)
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
	if err := syncRoot(files.path); err != nil {
		return nil, err
	}
	if initialization != nil {
		if err := initialization.Complete(); err != nil {
			return nil, err
		}
	}
	h.raw = bytes.Clone(raw)
	return h, nil
}

func (files *historyFiles) verify(expected []byte) error {
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
	if err != nil || !os.SameFile(held, retained) {
		return errors.Join(errors.New("introduction history lease lost"), err)
	}
	raw, err := readHistory(files.path)
	if err != nil || !bytes.Equal(raw, expected) {
		return errors.Join(errors.New("introduction history changed or lost"), err)
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

func replaceHistory(path string, raw []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".introduction-snapshot-")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer func() { _ = os.Remove(temporary) }()
	_, writeErr := file.Write(raw)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		return err
	}
	return syncHistoryDirectory(filepath.Dir(path))
}
