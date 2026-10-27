package duty

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
)

// participationInspection is independently instantiated native read custody.
// Source and Node keep separate producer policies over these original bytes.
type participationInspection struct {
	mu, ioMu           sync.Mutex
	users              sync.WaitGroup
	once               sync.Once
	ctx                context.Context
	path               string
	directory, lease   *os.File
	info               os.FileInfo
	files              map[string]*inspectionLeaf
	parents            map[string]*os.File
	parentInfo         map[string]os.FileInfo
	names              []string
	uid, gid           uint32
	closed             bool
	terminal, closeErr error
	observe            func(string, *os.File) // failure/hold only, after actual I/O
}
type inspectionLeaf struct {
	file   *os.File
	info   os.FileInfo
	digest [32]byte
}

func openParticipationInspection(ctx context.Context, path string, uid, gid uint32) (result *participationInspection, err error) {
	if ctx == nil || ctx.Err() != nil || !filepath.IsAbs(path) || filepath.Clean(path) != path || os.Geteuid() != 0 {
		return nil, errors.New("invalid retained participation inspection")
	}
	s := &participationInspection{ctx: ctx, path: path, files: map[string]*inspectionLeaf{}, parents: map[string]*os.File{}, parentInfo: map[string]os.FileInfo{}, uid: uid, gid: gid}
	defer func() {
		if err != nil {
			err = errors.Join(err, s.Close())
			result = nil
		}
	}()
	for path := filepath.Dir(path); ; path = filepath.Dir(path) {
		file, e := os.OpenFile(path, os.O_RDONLY|syscall.O_CLOEXEC|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
		if e != nil {
			return nil, e
		}
		s.parents[path] = file
		info, e := file.Stat()
		named, ne := os.Lstat(path)
		if e != nil || ne != nil || !os.SameFile(info, named) || !info.IsDir() || info.Mode().Perm()&0022 != 0 || info.Sys().(*syscall.Stat_t).Uid != 0 {
			return nil, errors.Join(errors.New("retained participation ancestor differs"), e, ne)
		}
		s.parentInfo[path] = info
		if path == "/" {
			break
		}
	}
	s.directory, err = os.OpenFile(path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	s.info, err = s.directory.Stat()
	if err != nil || s.info.Mode() != os.ModeDir|0700 || s.info.Sys().(*syscall.Stat_t).Uid != uid || s.info.Sys().(*syscall.Stat_t).Gid != gid {
		return nil, errors.Join(errors.New("retained participation root differs"), err)
	}
	fd, err := syscall.Openat(int(s.directory.Fd()), rootLockName, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	s.lease = os.NewFile(uintptr(fd), filepath.Join(path, rootLockName))
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, err
	}
	if err := verifyRootCandidate(path, false); err != nil {
		return nil, err
	}
	s.names, err = s.directory.Readdirnames(10)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(s.names) > 9 {
		return nil, errors.New("retained participation inventory exceeds bound")
	}
	slices.Sort(s.names)
	for _, name := range s.names {
		if strings.HasPrefix(name, ".stage-") || strings.HasPrefix(name, ".current-") || strings.HasPrefix(name, ".watermark-") {
			return nil, errors.New("retained participation has an uncertain prefix")
		}
		fd, err := syscall.Openat(int(s.directory.Fd()), name, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil, err
		}
		leaf := &inspectionLeaf{file: os.NewFile(uintptr(fd), filepath.Join(path, name))}
		s.files[name] = leaf
		leaf.info, err = leaf.file.Stat()
		if err != nil || leaf.info.Mode() != 0600 || leaf.info.Size() < 0 || leaf.info.Size() > maximumStateBytes {
			return nil, errors.Join(errors.New("retained participation leaf differs"), err)
		}
		native := leaf.info.Sys().(*syscall.Stat_t)
		if native.Uid != uid || native.Gid != gid || native.Nlink != 1 {
			return nil, errors.New("retained participation owner differs")
		}
		h := sha256.New()
		n, err := io.Copy(h, io.LimitReader(leaf.file, leaf.info.Size()+1))
		if err != nil || n != leaf.info.Size() {
			return nil, errors.Join(errors.New("retained participation bounded read differs"), err)
		}
		copy(leaf.digest[:], h.Sum(nil))
	}
	leaseInfo, err := s.lease.Stat()
	if err != nil || s.files[rootLockName] == nil || !os.SameFile(s.files[rootLockName].info, leaseInfo) {
		return nil, errors.Join(errors.New("retained participation lease differs"), err)
	}
	return s, nil
}

var errParticipationInspectionClosed = errors.New("retained participation inspection closed")

// The private policy is pure owning-domain code, never a caller callback or a
// runtime permission. Each public inspection selects its one fixed policy.
func (s *participationInspection) check(policy func([]dutyRecord) error) (err error) {
	if s == nil {
		return errors.New("retained participation inspection absent")
	}
	s.mu.Lock()
	if s.closed || s.terminal != nil || s.ctx.Err() != nil {
		err = errors.Join(errParticipationInspectionClosed, s.terminal, s.ctx.Err())
		s.mu.Unlock()
		return err
	}
	s.users.Add(1)
	s.mu.Unlock()
	defer s.users.Done()
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	err = s.read(policy)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil && s.closed {
		err = errParticipationInspectionClosed
	}
	if err == nil {
		err = s.ctx.Err()
	}
	if err != nil && err != errParticipationInspectionClosed {
		s.terminal = errors.Join(s.terminal, err)
	}
	return err
}

func (s *participationInspection) local() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		if err := errors.Join(s.terminal, s.ctx.Err()); err != nil {
			return errors.Join(errParticipationInspectionClosed, err)
		}
		return errParticipationInspectionClosed
	}
	return errors.Join(s.terminal, s.ctx.Err())
}

func (s *participationInspection) read(policy func([]dutyRecord) error) error {
	if err := s.local(); err != nil {
		return err
	}
	if err := s.nativeRoot(); err != nil {
		return err
	}
	for path, file := range s.parents {
		opened, e := file.Stat()
		named, ne := os.Lstat(path)
		original := s.parentInfo[path]
		if e != nil || ne != nil || !os.SameFile(original, opened) || !os.SameFile(original, named) || original.Mode() != named.Mode() || named.Sys().(*syscall.Stat_t).Uid != 0 || original.Sys().(*syscall.Stat_t).Gid != named.Sys().(*syscall.Stat_t).Gid {
			return errors.Join(errors.New("retained participation original ancestor lost"), e, ne)
		}
	}
	opened, e := s.directory.Stat()
	named, ne := os.Lstat(s.path)
	if e != nil || ne != nil || !sameInspectionFile(s.info, opened) || !sameInspectionFile(s.info, named) {
		return errors.Join(errors.New("retained participation original root lost"), e, ne)
	}
	if _, err := s.directory.Seek(0, io.SeekStart); err != nil {
		return err
	}
	names, e := s.directory.Readdirnames(10)
	if e != nil && e != io.EOF {
		return e
	}
	slices.Sort(names)
	if !slices.Equal(names, s.names) {
		return errors.New("retained participation original inventory changed")
	}
	for name, leaf := range s.files {
		opened, e := leaf.file.Stat()
		named, ne := os.Lstat(filepath.Join(s.path, name))
		if e != nil || ne != nil || !sameInspectionFile(leaf.info, opened) || !sameInspectionFile(leaf.info, named) {
			return errors.Join(errors.New("retained participation original leaf lost"), e, ne)
		}
		if _, err := leaf.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		h := sha256.New()
		n, e := io.Copy(h, io.LimitReader(leaf.file, leaf.info.Size()+1))
		if e != nil || n != leaf.info.Size() || !bytes.Equal(h.Sum(nil), leaf.digest[:]) {
			return errors.Join(errors.New("retained participation bytes changed"), e)
		}
	}
	watermark, name, exists, err := loadWatermark(s.path)
	if err != nil || !exists {
		return errors.Join(errors.New("retained participation watermark absent"), err)
	}
	pointer, err := readBounded(filepath.Join(s.path, "current"), 65)
	if err != nil || string(pointer) != name+"\n" {
		return errors.Join(errors.New("retained participation requires explicit recovery"), err)
	}
	state, err := loadGeneration(s.path, name)
	if err != nil || state.Generation != watermark {
		return errors.Join(errors.New("retained participation floor differs"), err)
	}
	if err := policy(state.Duties); err != nil {
		return err
	}
	if s.observe != nil {
		s.observe("read", s.directory)
	}
	if err := s.local(); err != nil {
		return err
	}
	// Semantic reads may yield. Original native bytes/identities still have to
	// survive the final handoff; no clock expiry clears a live producer.
	for name, leaf := range s.files {
		named, e := os.Lstat(filepath.Join(s.path, name))
		opened, oe := leaf.file.Stat()
		if e != nil || oe != nil || !sameInspectionFile(leaf.info, named) || !sameInspectionFile(leaf.info, opened) {
			return errors.Join(errors.New("retained participation late substitution"), e, oe)
		}
	}
	opened, e = s.directory.Stat()
	named, ne = os.Lstat(s.path)
	if e != nil || ne != nil || !sameInspectionFile(s.info, opened) || !sameInspectionFile(s.info, named) {
		return errors.Join(errors.New("retained participation late root changed"), e, ne)
	}
	return s.nativeRoot()
}

func (s *participationInspection) nativeRoot() (returned error) {
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(s.directory.Fd()), &fs); err != nil || fs.Type != 0xef53 || fs.Flags&1 != 0 {
		return errors.Join(errors.New("retained participation filesystem differs"), err)
	}
	named, err := os.OpenFile(s.path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer func() {
		if err := named.Close(); err != nil {
			returned = errors.Join(returned, err)
		}
	}()
	info, err := named.Stat()
	fsErr := syscall.Fstatfs(int(named.Fd()), &fs)
	if err != nil || fsErr != nil || !os.SameFile(s.info, info) || info.Mode() != s.info.Mode() || fs.Type != 0xef53 || fs.Flags&1 != 0 {
		return errors.Join(errors.New("retained participation named filesystem differs"), err, fsErr)
	}
	return s.local()
}

func sameInspectionFile(a, b os.FileInfo) bool {
	if a == nil || b == nil || !os.SameFile(a, b) || a.Mode() != b.Mode() || a.Size() != b.Size() {
		return false
	}
	x, xok := a.Sys().(*syscall.Stat_t)
	y, yok := b.Sys().(*syscall.Stat_t)
	return xok && yok && x.Uid == y.Uid && x.Gid == y.Gid && x.Nlink == y.Nlink && x.Ctim == y.Ctim && x.Mtim == y.Mtim
}

func (s *participationInspection) Close() error {
	if s == nil {
		return nil
	}
	s.once.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.users.Wait()
		var physical error
		closeFile := func(file *os.File) {
			if file == nil {
				return
			}
			if s.observe != nil {
				s.observe("before-close", file)
			}
			physical = errors.Join(physical, file.Close())
		}
		for _, leaf := range s.files {
			closeFile(leaf.file)
		}
		for _, file := range s.parents {
			closeFile(file)
		}
		closeFile(s.directory)
		closeFile(s.lease)
		s.closeErr = errors.Join(s.terminal, physical, s.ctx.Err())
	})
	return s.closeErr
}
