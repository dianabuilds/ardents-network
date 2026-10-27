package forwarding

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

var errCompletedClosed = errors.New("forwarding installation: completed observation closed")

// completedFiles owns fresh read-only descriptors; it never borrows Inputs or
// the creator's writer, native maps, pending inode or completion result. Root's
// separate completed reader assigns its exclusive journal observation and keeps
// it through all other closes. Runtime constructs its own fresh instance without
// any journal; this physical read machinery grants neither caller a live right.
type completedFiles struct {
	ctx              context.Context
	mu, ioMu         sync.Mutex
	uses             sync.WaitGroup
	once             sync.Once
	closed           bool
	failure          error
	files            map[string]*initialFile
	fileOrder        []*initialFile
	dirs             map[string]*inputDirectory
	dirOrder         []*inputDirectory
	journal          *initialFile
	clock            string
	pending          string
	sealed           string
	credential       string
	absentPaths      []string
	extraInventories map[string]int
	// Negative native controls only, after actual I/O or before actual Close.
	observe func(string, *os.File)
}

func (s *completedFiles) begin() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != nil {
		return s.failure
	}
	if err := s.ctx.Err(); err != nil {
		if !s.closed {
			s.failure = err
		}
		return err
	}
	if s.closed {
		return errCompletedClosed
	}
	s.uses.Add(1)
	return nil
}

func (s *completedFiles) local() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != nil {
		return s.failure
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if s.closed {
		return errCompletedClosed
	}
	return nil
}

func (s *completedFiles) record(err error) {
	if err == nil || err == errCompletedClosed {
		return
	}
	s.mu.Lock()
	if s.failure == nil {
		s.failure = err
	}
	s.mu.Unlock()
}

func (s *completedFiles) parents(path string) (*inputDirectory, error) {
	if !validPath(path) {
		return nil, ErrInput
	}
	parts := append([]string{""}, strings.Split(strings.TrimPrefix(filepath.Dir(path), "/"), "/")...)
	current := "/"
	var parent *inputDirectory
	for i, name := range parts {
		if i > 0 && name == "" {
			continue
		}
		if i > 0 {
			current = filepath.Join(current, name)
		}
		if original := s.dirs[current]; original != nil {
			if !trustedInputDirectory(original.info) {
				return nil, ErrBinding
			}
			parent = original
			continue
		}
		var file *os.File
		var err error
		if parent == nil {
			file, err = os.OpenFile("/", os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
		} else {
			file, err = openInputAt(parent, name, true)
		}
		if err != nil {
			return nil, err
		}
		original := &inputDirectory{path: current, name: name, file: file, parent: parent}
		s.dirs[current] = original
		s.dirOrder = append(s.dirOrder, original)
		original.info, err = file.Stat()
		if err != nil || !trustedInputDirectory(original.info) {
			return nil, errors.Join(ErrBinding, err)
		}
		parent = original
	}
	return parent, s.local()
}

func (s *completedFiles) directory(path string, mode os.FileMode, uid, gid uint32) error {
	parent, err := s.parents(path)
	if err != nil {
		return err
	}
	if original := s.dirs[path]; original != nil {
		if !nodeAccess(original.info, mode, uid, gid, true) {
			return ErrBinding
		}
		return s.local()
	}
	file, err := openInputAt(parent, filepath.Base(path), true)
	if err != nil {
		return err
	}
	original := &inputDirectory{path: path, name: filepath.Base(path), file: file, parent: parent}
	s.dirs[path] = original
	s.dirOrder = append(s.dirOrder, original)
	original.info, err = file.Stat()
	var fs syscall.Statfs_t
	fsErr := syscall.Fstatfs(int(file.Fd()), &fs)
	if err != nil || fsErr != nil || fs.Type != 0xef53 || fs.Flags&1 != 0 || !nodeAccess(original.info, mode, uid, gid, true) {
		return errors.Join(ErrBinding, err, fsErr)
	}
	return s.local()
}

func (s *completedFiles) retain(path string, maximum int64, mode os.FileMode, uid, gid uint32) (*initialFile, error) {
	if s.files[path] != nil {
		return nil, ErrInput
	}
	parent, err := s.parents(path)
	if err != nil {
		return nil, err
	}
	file, err := openInputAt(parent, filepath.Base(path), false)
	if err != nil {
		return nil, err
	}
	original := &initialFile{path: path, file: file}
	s.files[path] = original
	s.fileOrder = append(s.fileOrder, original)
	original.info, err = file.Stat()
	var fs syscall.Statfs_t
	fsErr := syscall.Fstatfs(int(file.Fd()), &fs)
	if err != nil || fsErr != nil || fs.Type != 0xef53 || !nodeAccess(original.info, mode, uid, gid, false) || original.info.Size() < 1 || original.info.Size() > maximum {
		return nil, errors.Join(ErrBinding, err, fsErr)
	}
	return original, s.local()
}

func (s *completedFiles) read(path string, maximum int64, mode os.FileMode, uid, gid uint32) ([]byte, error) {
	original, err := s.retain(path, maximum, mode, uid, gid)
	if err != nil {
		return nil, err
	}
	return s.fill(original, maximum)
}

func (s *completedFiles) fill(original *initialFile, maximum int64) ([]byte, error) {
	if err := s.local(); err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(io.LimitReader(original.file, maximum+1))
	original.body = raw
	if err != nil || int64(len(raw)) != original.info.Size() {
		return nil, errors.Join(ErrBinding, err)
	}
	if s.observe != nil {
		s.observe("read", original.file)
	}
	return raw, s.local()
}

func (s *completedFiles) absent(path string) error {
	parent := s.dirs[filepath.Dir(path)]
	if parent == nil {
		return ErrBinding
	}
	file, err := openInputAt(parent, filepath.Base(path), false)
	if file != nil {
		return errors.Join(ErrRepairRequired, file.Close())
	}
	if !errors.Is(err, syscall.ENOENT) {
		return errors.Join(ErrBinding, err)
	}
	return s.local()
}

func (s *completedFiles) paths() error {
	for _, original := range s.dirOrder {
		current, err := original.file.Stat()
		named, nameErr := os.Lstat(original.path)
		before := original.info
		if before == nil {
			return errors.Join(ErrBinding, err, nameErr)
		}
		native := before.Sys().(*syscall.Stat_t)
		if err != nil || nameErr != nil || !os.SameFile(before, current) || !os.SameFile(before, named) || !nodeAccess(current, before.Mode().Perm(), native.Uid, native.Gid, true) || !nodeAccess(named, before.Mode().Perm(), native.Uid, native.Gid, true) {
			return errors.Join(ErrBinding, err, nameErr)
		}
		if original.parent != nil {
			linked, err := openInputAt(original.parent, original.name, true)
			if err != nil {
				return err
			}
			info, statErr := linked.Stat()
			if err := errors.Join(statErr, linked.Close()); err != nil || !os.SameFile(before, info) || !nodeAccess(info, before.Mode().Perm(), native.Uid, native.Gid, true) {
				return errors.Join(ErrBinding, err)
			}
		}
	}
	for _, original := range s.fileOrder {
		info, err := original.file.Stat()
		if err != nil || !sameInputFile(original.info, info) {
			return errors.Join(ErrBinding, err)
		}
		parent := s.dirs[filepath.Dir(original.path)]
		linked, err := openInputAt(parent, filepath.Base(original.path), false)
		if err != nil {
			return err
		}
		named, statErr := linked.Stat()
		if err := errors.Join(statErr, linked.Close()); err != nil || !sameInputFile(original.info, named) {
			return errors.Join(ErrBinding, err)
		}
	}
	if err := s.absent(s.pending); err != nil {
		return err
	}
	for _, path := range s.absentPaths {
		if err := s.absent(path); err != nil {
			return err
		}
	}
	parent := s.dirs[filepath.Dir(s.clock)]
	clock, err := openInputAt(parent, filepath.Base(s.clock), false)
	if err != nil {
		return err
	}
	info, statErr := clock.Stat()
	named, nameErr := os.Lstat(s.clock)
	err = errors.Join(statErr, nameErr, clock.Close())
	if err != nil || !os.SameFile(info, named) || !nodeAccess(info, 0644, 0, 0, false) || !nodeAccess(named, 0644, 0, 0, false) {
		return errors.Join(ErrBinding, err)
	}
	return s.inventory()
}

func (s *completedFiles) inventory() error {
	inventories := map[string]int{s.sealed: 12, s.credential: 2}
	for path, count := range s.extraInventories {
		inventories[path] = count
	}
	for path, count := range inventories {
		dir := s.dirs[path]
		if dir == nil {
			return ErrBinding
		}
		if _, err := dir.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		entries, err := dir.file.ReadDir(count + 1)
		if err != nil && err != io.EOF || len(entries) != count {
			return errors.Join(ErrBinding, err)
		}
		for _, entry := range entries {
			if s.files[filepath.Join(path, entry.Name())] == nil || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				return ErrBinding
			}
		}
	}
	return nil
}

func (s *completedFiles) check() error {
	if err := s.local(); err != nil {
		return err
	}
	if err := s.paths(); err != nil {
		return err
	}
	var scratch [32 << 10]byte
	defer clear(scratch[:])
	for _, original := range s.fileOrder {
		for offset := 0; offset < len(original.body); {
			if err := s.local(); err != nil {
				return err
			}
			end := offset + min(len(scratch), len(original.body)-offset)
			n, err := original.file.ReadAt(scratch[:end-offset], int64(offset))
			if err != nil || n != end-offset || !bytes.Equal(scratch[:n], original.body[offset:end]) {
				return errors.Join(ErrBinding, err)
			}
			offset = end
		}
		n, err := original.file.ReadAt(scratch[:1], int64(len(original.body)))
		if n != 0 || err != io.EOF {
			return errors.Join(ErrBinding, err)
		}
		if s.observe != nil {
			s.observe("read", original.file)
		}
		if err := s.local(); err != nil {
			return err
		}
	}
	return errors.Join(s.paths(), s.local())
}

func (s *completedFiles) close() error {
	if s == nil {
		return nil
	}
	s.once.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		if s.observe != nil {
			s.observe("denied", nil)
		}
		s.uses.Wait()
		var physical error
		closeFile := func(original *initialFile) {
			if s.observe != nil {
				s.observe("before-close", original.file)
			}
			physical = errors.Join(physical, original.file.Close())
			clear(original.body)
		}
		for i := len(s.fileOrder) - 1; i >= 0; i-- {
			if s.fileOrder[i] != s.journal {
				closeFile(s.fileOrder[i])
			}
		}
		for i := len(s.dirOrder) - 1; i >= 0; i-- {
			physical = errors.Join(physical, s.dirOrder[i].file.Close())
		}
		if s.journal != nil {
			closeFile(s.journal)
		}
		s.mu.Lock()
		s.failure = errors.Join(s.failure, physical, s.ctx.Err())
		s.mu.Unlock()
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failure
}
