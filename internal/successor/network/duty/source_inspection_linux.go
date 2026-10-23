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

// SourceInspection retains its own existing participation lease and original
// flat files. It neither creates/repairs history nor removes a live producer.
type SourceInspection struct {
	mu               sync.Mutex
	ctx              context.Context
	path             string
	directory, lease *os.File
	info             os.FileInfo
	files            map[string]*sourceInspectionLeaf
	parents          map[string]*os.File
	parentInfo       map[string]os.FileInfo
	names            []string
	uid, gid         uint32
	closed           bool
	terminal         error
}
type sourceInspectionLeaf struct {
	file   *os.File
	info   os.FileInfo
	digest [32]byte
}

func OpenSourceInspection(ctx context.Context, path string, uid, gid uint32) (result *SourceInspection, err error) {
	if ctx == nil || ctx.Err() != nil || !filepath.IsAbs(path) || filepath.Clean(path) != path || os.Geteuid() != 0 {
		return nil, errors.New("invalid Source participation inspection")
	}
	s := &SourceInspection{ctx: ctx, path: path, files: map[string]*sourceInspectionLeaf{}, parents: map[string]*os.File{}, parentInfo: map[string]os.FileInfo{}, uid: uid, gid: gid}
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
			return nil, errors.Join(errors.New("Source participation ancestor differs"), e, ne)
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
		return nil, errors.Join(errors.New("Source participation root differs"), err)
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
		return nil, errors.New("Source participation inventory exceeds bound")
	}
	slices.Sort(s.names)
	for _, name := range s.names {
		if strings.HasPrefix(name, ".stage-") || strings.HasPrefix(name, ".current-") || strings.HasPrefix(name, ".watermark-") {
			return nil, errors.New("Source participation has an uncertain prefix")
		}
		fd, err := syscall.Openat(int(s.directory.Fd()), name, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil, err
		}
		leaf := &sourceInspectionLeaf{file: os.NewFile(uintptr(fd), filepath.Join(path, name))}
		s.files[name] = leaf
		leaf.info, err = leaf.file.Stat()
		if err != nil || !leaf.info.Mode().IsRegular() || leaf.info.Mode().Perm() != 0600 || leaf.info.Size() < 0 || leaf.info.Size() > maximumStateBytes {
			return nil, errors.Join(errors.New("Source participation leaf differs"), err)
		}
		native := leaf.info.Sys().(*syscall.Stat_t)
		if native.Uid != uid || native.Gid != gid || native.Nlink != 1 {
			return nil, errors.New("Source participation owner differs")
		}
		h := sha256.New()
		n, err := io.Copy(h, io.LimitReader(leaf.file, leaf.info.Size()+1))
		if err != nil || n != leaf.info.Size() {
			return nil, errors.Join(errors.New("Source participation bounded read differs"), err)
		}
		copy(leaf.digest[:], h.Sum(nil))
	}
	leaseInfo, err := s.lease.Stat()
	if err != nil || s.files[rootLockName] == nil || !os.SameFile(s.files[rootLockName].info, leaseInfo) {
		return nil, errors.Join(errors.New("Source participation lease differs"), err)
	}
	return s, s.Check()
}

func (s *SourceInspection) Check() (err error) {
	if s == nil {
		return errors.New("Source participation inspection absent")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() {
		if err != nil {
			s.terminal = errors.Join(s.terminal, err)
		}
	}()
	if s.closed || s.terminal != nil {
		return errors.Join(errors.New("Source participation inspection closed"), s.terminal)
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	for path, file := range s.parents {
		opened, e := file.Stat()
		named, ne := os.Lstat(path)
		original := s.parentInfo[path]
		if e != nil || ne != nil || !os.SameFile(original, opened) || !os.SameFile(original, named) || original.Mode() != named.Mode() || named.Sys().(*syscall.Stat_t).Uid != 0 || original.Sys().(*syscall.Stat_t).Gid != named.Sys().(*syscall.Stat_t).Gid {
			return errors.Join(errors.New("Source participation original ancestor lost"), e, ne)
		}
	}
	opened, e := s.directory.Stat()
	named, ne := os.Lstat(s.path)
	if e != nil || ne != nil || !sameSourceInspectionFile(s.info, opened) || !sameSourceInspectionFile(s.info, named) {
		return errors.Join(errors.New("Source participation original root lost"), e, ne)
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
		return errors.New("Source participation original inventory changed")
	}
	for name, leaf := range s.files {
		opened, e := leaf.file.Stat()
		named, ne := os.Lstat(filepath.Join(s.path, name))
		if e != nil || ne != nil || !sameSourceInspectionFile(leaf.info, opened) || !sameSourceInspectionFile(leaf.info, named) {
			return errors.Join(errors.New("Source participation original leaf lost"), e, ne)
		}
		if _, err := leaf.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		h := sha256.New()
		n, e := io.Copy(h, io.LimitReader(leaf.file, leaf.info.Size()+1))
		if e != nil || n != leaf.info.Size() || !bytes.Equal(h.Sum(nil), leaf.digest[:]) {
			return errors.Join(errors.New("Source participation bytes changed"), e)
		}
	}
	watermark, name, exists, err := loadWatermark(s.path)
	if err != nil || !exists {
		return errors.Join(errors.New("Source participation watermark absent"), err)
	}
	pointer, err := readBounded(filepath.Join(s.path, "current"), 65)
	if err != nil || string(pointer) != name+"\n" {
		return errors.Join(errors.New("Source participation requires explicit recovery"), err)
	}
	state, err := loadGeneration(s.path, name)
	if err != nil || state.Generation != watermark {
		return errors.Join(errors.New("Source participation floor differs"), err)
	}
	for _, record := range state.Duties {
		if record.Class != "direct-source" || record.State != "exposed" {
			return errors.New("Source participation has an unresolved original producer")
		}
	}
	// Semantic reads may yield. Original native bytes/identities still have to
	// survive the final handoff; no clock expiry clears a live producer.
	for name, leaf := range s.files {
		named, e := os.Lstat(filepath.Join(s.path, name))
		opened, oe := leaf.file.Stat()
		if e != nil || oe != nil || !sameSourceInspectionFile(leaf.info, named) || !sameSourceInspectionFile(leaf.info, opened) {
			return errors.Join(errors.New("Source participation late substitution"), e, oe)
		}
	}
	opened, e = s.directory.Stat()
	named, ne = os.Lstat(s.path)
	if e != nil || ne != nil || !sameSourceInspectionFile(s.info, opened) || !sameSourceInspectionFile(s.info, named) {
		return errors.Join(errors.New("Source participation late root changed"), e, ne)
	}
	return s.ctx.Err()
}

func sameSourceInspectionFile(a, b os.FileInfo) bool {
	if a == nil || b == nil || !os.SameFile(a, b) || a.Mode() != b.Mode() || a.Size() != b.Size() {
		return false
	}
	x, xok := a.Sys().(*syscall.Stat_t)
	y, yok := b.Sys().(*syscall.Stat_t)
	return xok && yok && x.Uid == y.Uid && x.Gid == y.Gid && x.Nlink == y.Nlink && x.Ctim == y.Ctim && x.Mtim == y.Mtim
}

func (s *SourceInspection) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return s.terminal
	}
	s.closed = true
	for _, leaf := range s.files {
		s.terminal = errors.Join(s.terminal, leaf.file.Close())
	}
	for _, file := range s.parents {
		s.terminal = errors.Join(s.terminal, file.Close())
	}
	if s.directory != nil {
		s.terminal = errors.Join(s.terminal, s.directory.Close())
	}
	if s.lease != nil {
		s.terminal = errors.Join(s.terminal, syscall.Flock(int(s.lease.Fd()), syscall.LOCK_UN), s.lease.Close())
	}
	return s.terminal
}
