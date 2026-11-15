package introduction

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Inspection independently retains complete stopped slot history. It exposes
// neither a Registry nor a mutable History and grants no registration, fresh
// root, runtime or Installation authority. Copies share denial and joined Close.
type Inspection struct{ original *slotInspection }

type slotInspection struct {
	mu, ioMu          sync.Mutex
	users             sync.WaitGroup
	once              sync.Once
	ctx               context.Context
	uid, gid          uint32
	parents           []*slotInspectionParent
	files             *historyFiles
	raw               []byte
	closed            bool
	failure, closeErr error
	// Negative controls after genuine reads or immediately before native Close.
	observe func(string, *os.File)
}

type slotInspectionParent struct {
	path string
	file *os.File
	info os.FileInfo
}

var errInspectionClosed = errors.New("stopped Introduction history inspection closed")

// OpenInspection opens existing Linux/ext4 history with its own read-only
// exclusive lease and exact numeric principal. It never creates, repairs,
// prunes, synchronizes or publishes history. Installation must independently
// prove original retirement and fresh Release permission for its own effects.
func OpenInspection(ctx context.Context, root string, binding Binding, uid, gid uint32) (result Inspection, returned error) {
	if ctx == nil || os.Geteuid() != 0 || uid < 100 || uid > 999 || gid != uid || !filepath.IsAbs(root) || filepath.Clean(root) != root || root == "/" || binding.NetworkID == [32]byte{} || binding.ProfileDigest == [32]byte{} || binding.ReceiverNodeID == [32]byte{} || binding.ReceiverDutyGeneration == 0 {
		return result, errors.New("stopped Introduction inspection input invalid")
	}
	if _, finite := ctx.Deadline(); !finite {
		return result, errors.New("stopped Introduction inspection requires finite caller")
	}
	s := &slotInspection{ctx: ctx, uid: uid, gid: gid, files: &historyFiles{rootPath: root, path: filepath.Join(root, slotHistoryName), watch: -1}}
	defer func() {
		if returned != nil {
			s.failure = returned
			returned = errors.Join(returned, s.close())
		}
	}()
	for path := filepath.Dir(root); ; path = filepath.Dir(path) {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if err != nil {
			return result, err
		}
		p := &slotInspectionParent{path: path, file: file}
		s.parents = append(s.parents, p)
		p.info, err = file.Stat()
		if err != nil || !slotInspectionAccess(p.info, 0, 0, true, true) {
			return result, errors.Join(errors.New("stopped Introduction ancestor access differs"), err)
		}
		if path == "/" {
			break
		}
	}
	f := s.files
	named, err := os.Lstat(root)
	if err != nil || !slotInspectionAccess(named, uid, gid, true, false) {
		return result, errors.Join(errors.New("stopped Introduction root access differs"), err)
	}
	f.root, err = os.OpenFile(root, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return result, err
	}
	f.rootInfo, err = f.root.Stat()
	if err != nil || !sameHistoryFile(named, f.rootInfo) {
		return result, errors.Join(errors.New("stopped Introduction original root differs"), err)
	}
	anchored := fmt.Sprintf("/proc/self/fd/%d", f.root.Fd())
	f.watch, err = syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		return result, err
	}
	if _, err := syscall.InotifyAddWatch(f.watch, anchored, syscall.IN_MOVE_SELF|syscall.IN_DELETE_SELF|syscall.IN_UNMOUNT|syscall.IN_ONLYDIR); err != nil {
		return result, err
	}
	f.lease, err = os.OpenFile(filepath.Join(anchored, slotLeaseName), os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return result, err
	}
	f.leaseInfo, err = f.lease.Stat()
	if err != nil || !slotInspectionAccess(f.leaseInfo, uid, gid, false, false) || f.leaseInfo.Size() != 0 {
		return result, errors.Join(errors.New("stopped Introduction lease differs"), err)
	}
	if err := syscall.Flock(int(f.lease.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return result, err
	}
	s.raw, err = readHistory(filepath.Join(anchored, slotHistoryName))
	if err != nil {
		return result, err
	}
	snapshot := slotSnapshot{binding: binding, entries: make(map[[32]byte]time.Time)}
	err = snapshot.decode(s.raw)
	clear(snapshot.entries)
	if err != nil {
		return result, err
	}
	if err := f.retainSnapshot(s.raw); err != nil {
		return result, err
	}
	if err := s.check(); err != nil {
		return result, err
	}
	return Inspection{original: s}, nil
}

func slotInspectionAccess(info os.FileInfo, uid, gid uint32, directory, ancestor bool) bool {
	if info == nil || info.IsDir() != directory || !directory && !info.Mode().IsRegular() || info.Mode()&(os.ModeSymlink|os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != uid {
		return false
	}
	if ancestor {
		return info.Mode().Perm()&0022 == 0
	}
	mode := os.FileMode(0600)
	if directory {
		mode = 0700
	}
	return native.Gid == gid && info.Mode().Perm() == mode && (directory || native.Nlink == 1)
}

func (s *slotInspection) local() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.Join(errInspectionClosed, s.failure, s.ctx.Err())
	}
	return errors.Join(s.failure, s.ctx.Err())
}

func (s *slotInspection) check() error {
	if err := s.local(); err != nil {
		return err
	}
	for range 2 {
		for _, p := range s.parents {
			opened, err := p.file.Stat()
			named, nameErr := os.Lstat(p.path)
			if err != nil || nameErr != nil || !os.SameFile(p.info, opened) || !os.SameFile(p.info, named) || opened.Mode() != p.info.Mode() || named.Mode() != p.info.Mode() || !slotInspectionAccess(opened, 0, 0, true, true) || !slotInspectionAccess(named, 0, 0, true, true) {
				return errors.Join(errors.New("stopped Introduction original ancestor differs"), err, nameErr)
			}
		}
		f := s.files
		var fs syscall.Statfs_t
		if err := syscall.Fstatfs(int(f.root.Fd()), &fs); err != nil {
			return err
		}
		if fs.Type != 0xef53 || fs.Flags&1 != 0 {
			return errors.New("stopped Introduction writable ext4 absent")
		}
		if !slotInspectionAccess(f.rootInfo, s.uid, s.gid, true, false) || !slotInspectionAccess(f.leaseInfo, s.uid, s.gid, false, false) || !slotInspectionAccess(f.snapshotInfo, s.uid, s.gid, false, false) {
			return errors.New("stopped Introduction history access differs")
		}
		if err := f.verify(s.raw); err != nil {
			return err
		}
		if s.observe != nil {
			s.observe("read", f.lease)
		}
		if err := s.local(); err != nil {
			return err
		}
	}
	return nil
}

// Check reobserves the original native history. A definite failure permanently
// denies this inspection even if bytes or names are subsequently restored.
func (h Inspection) Check() (returned error) {
	s := h.original
	if s == nil {
		return errInspectionClosed
	}
	s.mu.Lock()
	if s.closed || s.failure != nil || s.ctx.Err() != nil {
		returned = errors.Join(errInspectionClosed, s.failure, s.ctx.Err())
		s.mu.Unlock()
		return returned
	}
	s.users.Add(1)
	s.mu.Unlock()
	defer s.users.Done()
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	returned = s.check()
	s.mu.Lock()
	defer s.mu.Unlock()
	if returned == nil && s.closed {
		returned = errInspectionClosed
	}
	if returned == nil {
		returned = s.ctx.Err()
	}
	if returned != nil && returned != errInspectionClosed {
		s.failure = errors.Join(s.failure, returned)
	}
	return returned
}

// Close denies new reads, joins original readers and closes their native pins
// before returning the exclusive lease. Every copy retains the terminal result.
func (h Inspection) Close() error {
	if h.original == nil {
		return nil
	}
	return h.original.close()
}

func (s *slotInspection) close() error {
	s.once.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.users.Wait()
		f := s.files
		if s.observe != nil && f.lease != nil {
			s.observe("before-close", f.lease)
		}
		var physical error
		if f.snapshot != nil {
			physical = errors.Join(physical, f.snapshot.Close())
		}
		if f.watch >= 0 {
			physical = errors.Join(physical, syscall.Close(f.watch))
		}
		if f.root != nil {
			physical = errors.Join(physical, f.root.Close())
		}
		for i := len(s.parents) - 1; i >= 0; i-- {
			physical = errors.Join(physical, s.parents[i].file.Close())
		}
		if f.lease != nil {
			physical = errors.Join(physical, syscall.Flock(int(f.lease.Fd()), syscall.LOCK_UN), f.lease.Close())
		}
		clear(s.raw)
		s.closeErr = errors.Join(s.failure, physical, s.ctx.Err())
	})
	return s.closeErr
}
