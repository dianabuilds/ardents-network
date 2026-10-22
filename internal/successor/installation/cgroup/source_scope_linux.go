package cgroup

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"syscall"
)

const sourceScopePath = "/sys/fs/cgroup/system.slice/ardents-source.service"

// SourceScope independently retains the fixed original Source kernel scope.
// Its caller serializes observations, joins those users before Close, and owns
// process/manager admission and stopping. Joined reads only the original events
// descriptor; neither path absence nor another empty cgroup supplies completion.
type SourceScope struct{ original *sourceScope }

type sourceScope struct {
	ctx       context.Context
	pid       uint32
	kernel    *pin
	directory *os.File
	closed    bool
	closeErr  error
}

// RetainSourceStarted is root's physical observation after its own fixed Source
// start attempt. Partial custody survives an observation failure and must remain
// retained until original join. PID zero cannot supply successful admission.
func RetainSourceStarted(ctx context.Context, pid uint32) (*SourceScope, error) {
	if os.Geteuid() != 0 {
		return nil, errInput
	}
	return retainSourceScope(ctx, pid, true)
}

// RetainSourceStop independently observes an archived original scope before a
// new explicit stop. Before any interruption it is read-only observation;
// after an attempted effect Installation retains it through actual Joined.
func RetainSourceStop(ctx context.Context, pid uint32, device, inode uint64) (*SourceScope, error) {
	if os.Geteuid() != 0 || device == 0 || inode == 0 {
		return nil, errInput
	}
	scope, err := retainSourceScope(ctx, pid, true)
	if err != nil {
		return scope, err
	}
	native, ok := sourceScopeNative(scope.original.kernel.directory)
	if !ok || uint64(native.Dev) != device || native.Ino != inode {
		return scope, errBinding
	}
	return scope, nil
}

// RetainSourceSelf observes only this actual non-root calling process. It owns
// no root transition lease, stop right or foreign process selection.
func RetainSourceSelf(ctx context.Context) (*SourceScope, error) {
	if os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() {
		return nil, errInput
	}
	return retainSourceScope(ctx, uint32(os.Getpid()), false)
}

func retainSourceScope(ctx context.Context, pid uint32, partial bool) (result *SourceScope, err error) {
	if ctx == nil {
		return nil, errInput
	}
	s := &sourceScope{ctx: ctx, pid: pid}
	s.kernel, err = pinFixedScope(ctx, "ardents-source.service")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil && !partial {
			err = errors.Join(err, s.close())
			result = nil
		}
	}()
	s.directory, err = os.OpenFile(sourceScopePath, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return &SourceScope{original: s}, err
	}
	return &SourceScope{original: s}, s.observe()
}

// Observe checks the same original directory, sole process and absence of child
// scopes. It never follows another PID or adopts a replacement path.
func (s *SourceScope) Observe() error {
	if s == nil || s.original == nil {
		return errInput
	}
	return s.original.observe()
}

func (s *sourceScope) observe() (err error) {
	if s == nil || s.ctx == nil || s.closed || s.kernel == nil || s.directory == nil || s.pid == 0 {
		return errInput
	}
	defer func() { err = errors.Join(err, s.ctx.Err()) }()
	if err = s.ctx.Err(); err != nil {
		return err
	}
	if err = s.observeDirectory(); err != nil {
		return err
	}
	removed, _, err := s.kernel.read()
	if err != nil || removed {
		return errors.Join(errBinding, err)
	}
	fd, err := syscall.Openat(int(s.directory.Fd()), "cgroup.procs", syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), "original Source cgroup.procs")
	defer func() { err = errors.Join(err, file.Close()) }()
	info, err := file.Stat()
	var fs syscall.Statfs_t
	fsErr := syscall.Fstatfs(fd, &fs)
	native, ok := sourceScopeNative(info)
	if err != nil || fsErr != nil || fs.Type != cgroup2Magic || !ok || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 || native.Uid != 0 || native.Gid != 0 {
		return errors.Join(errBinding, err, fsErr)
	}
	body, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || !bytes.Equal(body, []byte(strconv.FormatUint(uint64(s.pid), 10)+"\n")) {
		return errors.Join(errBinding, err)
	}
	if _, err = s.directory.Seek(0, io.SeekStart); err != nil {
		return err
	}
	entries, err := s.directory.ReadDir(257)
	if err != nil && err != io.EOF || len(entries) > 256 {
		return errors.Join(errBinding, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return errBinding
		}
	}
	if err = s.observeDirectory(); err != nil {
		return err
	}
	removed, _, err = s.kernel.read()
	if err != nil || removed {
		return errors.Join(errBinding, err)
	}
	return nil
}

func (s *sourceScope) observeDirectory() error {
	opened, err := s.directory.Stat()
	var fs syscall.Statfs_t
	fsErr := syscall.Fstatfs(int(s.directory.Fd()), &fs)
	if err != nil || fsErr != nil || fs.Type != cgroup2Magic {
		return errors.Join(errBinding, err, fsErr)
	}
	named, namedErr := os.Lstat(sourceScopePath)
	if namedErr != nil ||
		!trustedDirectory(opened) || !trustedDirectory(named) || s.kernel.directory == nil ||
		!os.SameFile(s.kernel.directory, opened) || !os.SameFile(opened, named) {
		return errors.Join(errBinding, err, namedErr, fsErr)
	}
	return nil
}

func sourceScopeNative(info os.FileInfo) (*syscall.Stat_t, bool) {
	if info == nil {
		return nil, false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return native, ok
}

// Joined remains available for physical cleanup after original caller denial.
// It grants no accepting observation and never replaces original custody.
func (s *SourceScope) Joined() (bool, error) {
	if s == nil || s.original == nil {
		return false, errInput
	}
	return s.original.joined()
}

func (s *sourceScope) joined() (bool, error) {
	if s == nil || s.closed || s.kernel == nil {
		return false, errInput
	}
	return pinsJoined([]*pin{s.kernel})
}

// Close releases observations only; root must establish Joined first. Runtime
// closes its own read-only observer after its users join, before its process exits.
func (s *SourceScope) Close() error {
	if s == nil || s.original == nil {
		return nil
	}
	return s.original.close()
}

func (s *sourceScope) close() error {
	if s == nil {
		return nil
	}
	if !s.closed {
		s.closed = true
		if s.directory != nil {
			s.closeErr = s.directory.Close()
		}
		s.closeErr = errors.Join(s.closeErr, s.kernel.close())
	}
	return s.closeErr
}
