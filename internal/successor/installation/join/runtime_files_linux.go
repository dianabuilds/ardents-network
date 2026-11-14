package join

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
)

var errRuntimeFilesClosed = errors.New("JOIN installation: runtime files closed")

// runtimeFiles owns fresh read-only descriptors; it never borrows Inputs or
// the creator's writer, native maps, pending inode or completion result. Root's
// separate completed reader assigns its exclusive journal observation and keeps
// it through all other closes. Runtime constructs its own fresh instance without
// any journal; this physical read machinery grants neither caller a live right.
type runtimeFiles struct {
	ctx         context.Context
	mu, ioMu    sync.Mutex
	uses        sync.WaitGroup
	once        sync.Once
	closed      bool
	failure     error
	files       map[string]*initialFile
	fileOrder   []*initialFile
	executable  *runtimeExecutableView
	dirs        map[string]*inputDirectory
	dirOrder    []*inputDirectory
	journal     *initialFile
	clock       string
	pending     string
	sealed      string
	credential  string
	absentPaths []string
	// Negative native controls only, after actual I/O or before actual Close.
	observe func(string, *os.File)
}

func (s *runtimeFiles) parents(path string) (*inputDirectory, error) {
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

func (s *runtimeFiles) directory(path string, mode os.FileMode, uid, gid uint32) error {
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

func (s *runtimeFiles) retain(path string, maximum int64, mode os.FileMode, uid, gid uint32) (*initialFile, error) {
	if s.files[path] != nil {
		return nil, ErrInput
	}
	parent, err := s.parents(path)
	if err != nil {
		return nil, err
	}
	return s.retainAt(path, parent, maximum, mode, uid, gid)
}

// retainAt opens only beneath this reader's already retained exact directory.
// The caller must independently admit that directory's specific access policy;
// ordinary retain keeps its stricter root:root ancestor walk.
func (s *runtimeFiles) retainAt(path string, parent *inputDirectory, maximum int64, mode os.FileMode, uid, gid uint32) (*initialFile, error) {
	if !validPath(path) || s.files[path] != nil || parent == nil || parent.file == nil || parent.info == nil || parent.path != filepath.Dir(path) || s.dirs[parent.path] != parent {
		return nil, ErrInput
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

func (s *runtimeFiles) read(path string, maximum int64, mode os.FileMode, uid, gid uint32) ([]byte, error) {
	original, err := s.retain(path, maximum, mode, uid, gid)
	if err != nil {
		return nil, err
	}
	return s.fill(original, maximum)
}

func (s *runtimeFiles) fill(original *initialFile, maximum int64) ([]byte, error) {
	if err := s.local(); err != nil {
		return nil, err
	}
	if original == nil || original.file == nil || original.info == nil || original.info.Size() < 0 || original.info.Size() > maximum {
		return nil, ErrBinding
	}
	// Native custody already bounded this exact original size. Allocate once
	// rather than geometrically growing and copying the complete program image.
	// A separate read still proves exact EOF, including a file grown after Stat.
	raw := make([]byte, original.info.Size())
	original.body = raw
	for offset := 0; offset < len(raw); {
		if err := s.local(); err != nil {
			return nil, err
		}
		end := offset + min(64<<10, len(raw)-offset)
		if _, err := io.ReadFull(original.file, raw[offset:end]); err != nil {
			return nil, errors.Join(ErrBinding, err)
		}
		offset = end
		if s.observe != nil {
			s.observe("fill-chunk", original.file)
		}
		// Initial acquisition needs the same bounded physical reads as later
		// comparisons, without growing or copying the retained byte image.
		runtime.Gosched()
	}
	if err := s.local(); err != nil {
		return nil, err
	}
	var extra [1]byte
	n, err := original.file.Read(extra[:])
	if n != 0 || err != io.EOF {
		return nil, errors.Join(ErrBinding, err)
	}
	if s.observe != nil {
		s.observe("read", original.file)
	}
	return raw, s.local()
}

func (s *runtimeFiles) absent(path string) error {
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

func (s *runtimeFiles) links() error {
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
			named, err := statRuntimeName(original.parent, original.name, true)
			if err != nil {
				return err
			}
			if !sameRuntimeDirectory(before, named) {
				return ErrBinding
			}
		}
	}
	for _, original := range s.fileOrder {
		info, err := original.file.Stat()
		if err != nil || !sameInputFile(original.info, info) {
			return errors.Join(ErrBinding, err)
		}
		parent := s.dirs[filepath.Dir(original.path)]
		named, err := statRuntimeName(parent, filepath.Base(original.path), false)
		if err != nil {
			return err
		}
		if !sameRuntimeFile(original.info, named) {
			return ErrBinding
		}
	}
	return s.local()
}

func (s *runtimeFiles) paths() error {
	if err := s.links(); err != nil {
		return err
	}
	if err := s.absent(s.pending); err != nil {
		return err
	}
	for _, path := range s.absentPaths {
		if err := s.absent(path); err != nil {
			return err
		}
	}
	if err := checkRuntimeClockFile(s.dirs[filepath.Dir(s.clock)], s.clock); err != nil {
		return err
	}
	for path, count := range map[string]int{s.sealed: 12, s.credential: 2} {
		d := s.dirs[path]
		if d == nil {
			return ErrBinding
		}
		if _, err := d.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		entries, err := d.file.ReadDir(count + 1)
		if err != nil && err != io.EOF || len(entries) != count {
			return errors.Join(ErrBinding, err)
		}
		for _, entry := range entries {
			if s.files[filepath.Join(path, entry.Name())] == nil || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				return ErrBinding
			}
		}
	}
	return s.local()
}
