package forwarding

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// Concrete fresh native observations, never manager or artifact authority. Its
// serialized recovery caller independently retains and checks the actual fixed
// manager connection, fresh Release and complete original transition evidence.
type recoveryContext struct {
	ctx       context.Context
	expected  nativeContextSnapshot
	identity  *runtimeIdentity
	namespace *startNamespace
	dirs      map[string]*os.File
	order     []*os.File
	closed    bool
	terminal  error
	// Failure/hold-only control before actual native Close; never supplied facts.
	beforeClose func(*os.File) error
}

func nativeContextObject(path string, info os.FileInfo) (nativeObject, error) {
	native, ok := infoSys(info)
	if !ok || native.Dev == 0 || native.Ino == 0 {
		return nativeObject{}, ErrBinding
	}
	return nativeObject{path, uint64(native.Dev), native.Ino, uint32(info.Mode()), native.Uid, native.Gid}, nil
}

func openRecoveryContext(ctx context.Context, raw []byte) (result *recoveryContext, returned error) {
	if ctx == nil || os.Geteuid() != 0 || os.Getegid() != 0 {
		return nil, ErrInput
	}
	expected, err := decodeNativeContext(raw)
	if err != nil {
		return nil, err
	}
	s := &recoveryContext{ctx: ctx, expected: expected, dirs: map[string]*os.File{}}
	defer func() {
		if returned != nil {
			s.terminal = returned
			returned = s.close()
			result = nil
		}
	}()
	s.identity, err = openRootIdentity(ctx)
	if err != nil {
		return nil, err
	}
	s.namespace, err = openStartNamespace(ctx, expected.UnitPaths)
	if err != nil {
		return nil, err
	}
	for _, object := range expected.Ancestors {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// Every named parent must itself be independently retained. No symlink
		// traversal, replacement pathname or new ancestor can substitute a root.
		if object.Path != "/" && s.dirs[filepath.Dir(object.Path)] == nil {
			return nil, ErrBinding
		}
		file, err := os.OpenFile(object.Path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if err != nil {
			return nil, err
		}
		s.dirs[object.Path], s.order = file, append(s.order, file)
		if err := observeRecoveryDirectory(file, object); err != nil {
			return nil, err
		}
	}
	if err := s.check(); err != nil {
		return nil, err
	}
	return s, nil
}

func observeRecoveryDirectory(file *os.File, expected nativeObject) error {
	if file == nil {
		return ErrInput
	}
	opened, err := file.Stat()
	named, nameErr := os.Lstat(expected.Path)
	if err != nil || nameErr != nil || !opened.IsDir() || !named.IsDir() {
		return errors.Join(ErrBinding, err, nameErr)
	}
	a, ae := nativeContextObject(expected.Path, opened)
	b, be := nativeContextObject(expected.Path, named)
	if ae != nil || be != nil || a != expected || b != expected {
		return errors.Join(ErrBinding, ae, be)
	}
	return nil
}

func (s *recoveryContext) check() (returned error) {
	if s == nil || s.closed || s.ctx == nil {
		return ErrInput
	}
	if s.terminal != nil {
		return s.terminal
	}
	defer func() { s.terminal = errors.Join(s.terminal, returned) }()
	if err := errors.Join(s.ctx.Err(), s.identity.check(), s.namespace.check()); err != nil {
		return err
	}
	manager, err := nativeContextObject("/proc/1", s.identity.managerInfo)
	if err != nil || manager != s.expected.Process || s.identity.started != s.expected.Started || digest(s.identity.bootRaw) != s.expected.BootSHA256 {
		return errors.Join(ErrBinding, err)
	}
	for n, name := range []string{"mnt", "pid", "cgroup", "uts", "user"} {
		info, err := s.identity.namespaces[name].Stat()
		if err != nil {
			return err
		}
		object, err := nativeContextObject("/proc/self/ns/"+name, info)
		if err != nil || object != s.expected.Namespaces[n] {
			return errors.Join(ErrBinding, err)
		}
	}
	for _, object := range s.expected.Ancestors {
		if err := observeRecoveryDirectory(s.dirs[object.Path], object); err != nil {
			return err
		}
	}
	for _, original := range s.namespace.parents.dirs {
		file := s.dirs[original.path]
		if file == nil {
			return ErrBinding
		}
		info, err := file.Stat()
		if err != nil || !os.SameFile(info, original.info) || info.Mode() != original.info.Mode() {
			return errors.Join(ErrBinding, err)
		}
	}
	for _, absent := range s.expected.Absent {
		if _, err := os.Lstat(absent); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
	}
	return errors.Join(s.identity.check(), s.namespace.check(), s.ctx.Err())
}

func (s *recoveryContext) close() error {
	if s == nil {
		return nil
	}
	if !s.closed {
		s.closed = true
		closeFile := func(file *os.File) {
			if s.beforeClose != nil {
				s.terminal = errors.Join(s.terminal, s.beforeClose(file))
			}
			s.terminal = errors.Join(s.terminal, file.Close())
		}
		for n := len(s.order) - 1; n >= 0; n-- {
			closeFile(s.order[n])
		}
		s.terminal = errors.Join(s.terminal, s.namespace.close(), s.identity.close(), s.ctx.Err())
	}
	return s.terminal
}
