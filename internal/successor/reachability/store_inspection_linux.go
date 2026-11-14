package reachability

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// StoreInspection independently retains stopped Store history. It exports no
// proof, mutable Store, lease or descriptor, and grants no publication, lookup,
// ACK, runtime or Installation authority. Copies share denial and joined Close.
type StoreInspection struct{ original *storeInspection }

type storeInspection struct {
	mu, ioMu          sync.Mutex
	users             sync.WaitGroup
	once              sync.Once
	ctx               context.Context
	root              string
	uid, gid          uint32
	parents           []*storeInspectionParent
	custody           *storeCustody
	lease             storeLease
	closed            bool
	failure, closeErr error
	// Negative controls only, after genuine observation or before actual Close.
	observe func(string, *os.File)
}

type storeInspectionParent struct {
	path string
	file *os.File
	info os.FileInfo
}

var errStoreInspectionClosed = errors.New("stopped Store inspection closed")

// OpenStoreInspection reads complete existing Linux/ext4 history under a fresh
// read-only exclusive lease and the exact numeric Node principal. It creates,
// repairs, prunes, synchronizes and publishes nothing. Installation separately
// proves original domain/native retirement and fresh Release admission.
func OpenStoreInspection(ctx context.Context, config StoreConfig, uid, gid uint32) (StoreInspection, error) {
	return openStoreInspection(ctx, config, uid, gid, nil)
}

func openStoreInspection(ctx context.Context, config StoreConfig, uid, gid uint32, observe func(string, *os.File)) (result StoreInspection, returned error) {
	if ctx == nil || os.Geteuid() != 0 || uid < 100 || uid > 999 || gid != uid || config.Network == [32]byte{} || !filepath.IsAbs(config.Root) || filepath.Clean(config.Root) != config.Root || config.Root == "/" {
		return result, errors.New("stopped Store inspection input invalid")
	}
	if _, finite := ctx.Deadline(); !finite {
		return result, errors.New("stopped Store inspection requires finite caller")
	}
	s := &storeInspection{ctx: ctx, root: config.Root, uid: uid, gid: gid, observe: observe}
	defer func() {
		if returned != nil {
			s.failure = returned
			returned = errors.Join(returned, s.close())
		}
	}()
	for path := filepath.Dir(config.Root); ; path = filepath.Dir(path) {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if err != nil {
			return result, err
		}
		parent := &storeInspectionParent{path: path, file: file}
		s.parents = append(s.parents, parent)
		parent.info, err = file.Stat()
		if err != nil || !storeInspectionAccess(parent.info, 0, 0, true, true) {
			return result, errors.Join(errors.New("stopped Store ancestor access differs"), err)
		}
		if path == "/" {
			break
		}
	}
	named, err := os.Lstat(config.Root)
	if err != nil || !storeInspectionAccess(named, uid, gid, true, false) {
		return result, errors.Join(errors.New("stopped Store root access differs"), err)
	}
	root, err := os.OpenRoot(config.Root)
	if err != nil {
		return result, err
	}
	rootTransferred := false
	defer func() {
		if !rootTransferred {
			returned = errors.Join(returned, root.Close())
		}
	}()
	pinned, err := root.Stat(".")
	if err != nil || !sameStoreInspectionObject(named, pinned) {
		return result, errors.Join(errors.New("stopped Store root changed at opening"), err)
	}
	lock, err := root.OpenFile(storeLockName, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return result, err
	}
	s.lease.file = lock
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return result, err
	}
	if err := checkRetainedStoreRoot(root); err != nil {
		return result, err
	}
	// Native custody consumes this root, also joining it on partial failure.
	rootTransferred = true
	s.custody, err = retainStoreCustody(root)
	if err != nil {
		s.custody = nil
		return result, err
	}
	floors := make(map[[32]byte]retainedFloor)
	if err := restoreStoreFloors(s.custody, config.Network, floors, true); err != nil {
		clear(floors)
		return result, err
	}
	clear(floors)
	if err := s.check(); err != nil {
		return result, err
	}
	return StoreInspection{original: s}, nil
}

func storeInspectionAccess(info os.FileInfo, uid, gid uint32, directory, ancestor bool) bool {
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

func sameStoreInspectionObject(original, actual os.FileInfo) bool {
	if original == nil || actual == nil || !os.SameFile(original, actual) || original.Mode() != actual.Mode() {
		return false
	}
	a, aok := original.Sys().(*syscall.Stat_t)
	b, bok := actual.Sys().(*syscall.Stat_t)
	return aok && bok && a.Uid == b.Uid && a.Gid == b.Gid && (original.IsDir() || original.Size() == actual.Size() && a.Nlink == b.Nlink && a.Ctim == b.Ctim && a.Mtim == b.Mtim)
}

func (s *storeInspection) local() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.Join(errStoreInspectionClosed, s.failure, s.ctx.Err())
	}
	return errors.Join(s.failure, s.ctx.Err())
}

func (s *storeInspection) check() error {
	if err := s.local(); err != nil {
		return err
	}
	for round := 0; round < 2; round++ {
		for _, p := range s.parents {
			opened, err := p.file.Stat()
			named, nameErr := os.Lstat(p.path)
			if err != nil || nameErr != nil || !sameStoreInspectionObject(p.info, opened) || !sameStoreInspectionObject(p.info, named) || !storeInspectionAccess(named, 0, 0, true, true) {
				return errors.Join(errors.New("stopped Store original ancestor differs"), err, nameErr)
			}
		}
		if err := s.nativeRoot(); err != nil {
			return err
		}
		c := s.custody
		entries := []struct {
			path string
			info os.FileInfo
		}{{s.root, c.rootInfo}, {filepath.Join(s.root, storeRecords), c.recordsInfo}, {filepath.Join(s.root, storeMarkerName), c.markerInfo}, {filepath.Join(s.root, storeLockName), c.lockInfo}}
		for name, record := range c.files {
			entries = append(entries, struct {
				path string
				info os.FileInfo
			}{filepath.Join(s.root, storeRecords, name), record.info})
		}
		for _, entry := range entries {
			named, err := os.Lstat(entry.path)
			if err != nil || !sameStoreInspectionObject(entry.info, named) || !storeInspectionAccess(named, s.uid, s.gid, entry.info.IsDir(), false) {
				return errors.Join(errors.New("stopped Store original access or identity differs"), err)
			}
		}
		if err := checkRetainedStoreRoot(c.root); err != nil {
			return err
		}
		if err := inspectStoreCustody(c, &s.lease, s.root); err != nil {
			return err
		}
		if s.observe != nil {
			s.observe("read", s.lease.file)
		}
		if err := s.local(); err != nil {
			return err
		}
	}
	return nil
}

func (s *storeInspection) nativeRoot() (returned error) {
	for _, original := range []bool{true, false} {
		var root *os.File
		var err error
		if original {
			root, err = s.custody.root.Open(".")
		} else {
			root, err = os.OpenFile(s.root, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		}
		if err != nil {
			return err
		}
		var fs syscall.Statfs_t
		info, statErr := root.Stat()
		fsErr := syscall.Fstatfs(int(root.Fd()), &fs)
		closeErr := root.Close()
		if err := errors.Join(statErr, fsErr, closeErr); err != nil {
			return err
		}
		if !sameStoreInspectionObject(s.custody.rootInfo, info) || fs.Type != 0xef53 || fs.Flags&1 != 0 {
			return errors.New("stopped Store original writable ext4 differs")
		}
	}
	return nil
}

// Check reobserves complete original history under this inspection's own lease.
// A definite failure permanently denies the handle even after files are restored.
func (h StoreInspection) Check() (returned error) {
	s := h.original
	if s == nil {
		return errStoreInspectionClosed
	}
	s.mu.Lock()
	if s.closed || s.failure != nil || s.ctx.Err() != nil {
		returned = errors.Join(errStoreInspectionClosed, s.failure, s.ctx.Err())
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
		returned = errStoreInspectionClosed
	}
	if returned == nil {
		returned = s.ctx.Err()
	}
	if returned != nil && returned != errStoreInspectionClosed {
		s.failure = errors.Join(s.failure, returned)
	}
	return returned
}

// Close seals new checks and joins original readers, native closes and the
// read-only exclusive lease last. Every copy retains the same terminal result.
func (h StoreInspection) Close() error {
	if h.original == nil {
		return nil
	}
	return h.original.close()
}

func (s *storeInspection) close() error {
	s.once.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.users.Wait()
		if s.observe != nil && s.lease.file != nil {
			s.observe("before-close", s.lease.file)
		}
		physical := s.custody.close()
		for i := len(s.parents) - 1; i >= 0; i-- {
			physical = errors.Join(physical, s.parents[i].file.Close())
		}
		physical = errors.Join(physical, s.lease.release())
		s.closeErr = errors.Join(s.failure, physical, s.ctx.Err())
	})
	return s.closeErr
}
