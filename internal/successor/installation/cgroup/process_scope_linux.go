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

// processScope is physical kernel custody only. Each public fixed-purpose
// opener chooses its literal unit before acquisition; no public unit selector,
// role right, manager permission or domain completion crosses this mechanism.
type processScope struct {
	path      string
	ctx       context.Context
	pid       uint32
	kernel    *pin
	directory *os.File
	closed    bool
	closeErr  error
}

func retainProcessScope(ctx context.Context, pid uint32, partial bool, unit string) (result *processScope, err error) {
	if ctx == nil || unit != "ardents-source.service" && unit != "ardents-forwarding.service" {
		return nil, errInput
	}
	s := &processScope{ctx: ctx, pid: pid, path: "/sys/fs/cgroup/system.slice/" + unit}
	s.kernel, err = pinFixedScope(ctx, unit)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil && !partial {
			err = errors.Join(err, s.close())
			result = nil
		}
	}()
	s.directory, err = os.OpenFile(s.path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return s, err
	}
	return s, s.observe()
}

func (s *processScope) observe() (err error) {
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
	file := os.NewFile(uintptr(fd), "original fixed process cgroup.procs")
	defer func() { err = errors.Join(err, file.Close()) }()
	info, err := file.Stat()
	var fs syscall.Statfs_t
	fsErr := syscall.Fstatfs(fd, &fs)
	native, ok := scopeNative(info)
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

func (s *processScope) observeDirectory() error {
	opened, err := s.directory.Stat()
	var fs syscall.Statfs_t
	fsErr := syscall.Fstatfs(int(s.directory.Fd()), &fs)
	if err != nil || fsErr != nil || fs.Type != cgroup2Magic {
		return errors.Join(errBinding, err, fsErr)
	}
	named, namedErr := os.Lstat(s.path)
	if namedErr != nil ||
		!trustedDirectory(opened) || !trustedDirectory(named) || s.kernel.directory == nil ||
		!os.SameFile(s.kernel.directory, opened) || !os.SameFile(opened, named) {
		return errors.Join(errBinding, err, namedErr, fsErr)
	}
	return nil
}

func scopeNative(info os.FileInfo) (*syscall.Stat_t, bool) {
	if info == nil {
		return nil, false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return native, ok
}

func (s *processScope) joined() (bool, error) {
	if s == nil || s.closed || s.kernel == nil {
		return false, errInput
	}
	return pinsJoined([]*pin{s.kernel})
}

func (s *processScope) close() error {
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
