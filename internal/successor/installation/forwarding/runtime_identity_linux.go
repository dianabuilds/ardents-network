package forwarding

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// Runtime retains its own namespace objects and the original visible manager
// process/boot. PrivateTmp intentionally gives it a different mount namespace
// from root's manager; root's namespace-equality policy is not reused here.
type runtimeIdentity struct {
	ctx                      context.Context
	namespaces               map[string]*os.File
	identities               map[string]os.FileInfo
	manager                  *os.File
	managerInfo              os.FileInfo
	started                  uint64
	boot                     *os.File
	bootInfo                 os.FileInfo
	bootRaw                  []byte
	closed                   bool
	terminal                 error
	requireManagerNamespaces bool
}

func openRuntimeIdentity(ctx context.Context) (result *runtimeIdentity, err error) {
	if ctx == nil || os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() {
		return nil, ErrInput
	}
	return openKernelIdentity(ctx)
}

func openRootIdentity(ctx context.Context) (*runtimeIdentity, error) {
	if ctx == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	i, err := openKernelIdentity(ctx)
	if err != nil {
		return nil, err
	}
	i.requireManagerNamespaces = true
	if err := i.check(); err != nil {
		return nil, errors.Join(err, i.close())
	}
	return i, nil
}

func openKernelIdentity(ctx context.Context) (result *runtimeIdentity, err error) {
	i := &runtimeIdentity{ctx: ctx, namespaces: map[string]*os.File{}, identities: map[string]os.FileInfo{}}
	defer func() {
		if err != nil {
			i.terminal = err
			err = errors.Join(err, i.close())
			result = nil
		}
	}()
	for _, name := range []string{"mnt", "pid", "cgroup", "uts", "user"} {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		file, e := os.OpenFile("/proc/self/ns/"+name, os.O_RDONLY|syscall.O_CLOEXEC, 0)
		if e != nil {
			return nil, e
		}
		i.namespaces[name] = file
		i.identities[name], err = file.Stat()
		if err != nil {
			return nil, err
		}
	}
	i.manager, err = os.OpenFile("/proc/1", os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	i.managerInfo, err = i.manager.Stat()
	if err != nil {
		return nil, err
	}
	i.boot, err = os.OpenFile("/proc/sys/kernel/random/boot_id", os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	i.bootInfo, err = i.boot.Stat()
	if err != nil {
		return nil, err
	}
	i.bootRaw, err = readKernelFile(i.boot, 128)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSuffix(string(i.bootRaw), "\n")
	if len(text) != 36 || len(i.bootRaw) != 37 {
		return nil, ErrBinding
	}
	for index, c := range []byte(text) {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if c != '-' {
				return nil, ErrBinding
			}
		} else if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return nil, ErrBinding
		}
	}
	i.started, err = i.managerStart()
	if err != nil {
		return nil, err
	}
	return i, i.check()
}

func readKernelFile(file *os.File, maximum int64) (raw []byte, err error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	var fs syscall.Statfs_t
	fsErr := syscall.Fstatfs(int(file.Fd()), &fs)
	if !ok || native.Uid != 0 || !info.Mode().IsRegular() || fsErr != nil || fs.Type != 0x9fa0 {
		return nil, errors.Join(ErrBinding, fsErr)
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	raw, err = io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || len(raw) == 0 || int64(len(raw)) > maximum {
		return nil, errors.Join(ErrBinding, err)
	}
	return raw, nil
}

func (i *runtimeIdentity) managerStart() (started uint64, err error) {
	fd, err := syscall.Openat(int(i.manager.Fd()), "stat", syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return 0, err
	}
	file := os.NewFile(uintptr(fd), "original Forwarding manager stat")
	defer func() { err = errors.Join(err, file.Close(), i.ctx.Err()) }()
	raw, err := readKernelFile(file, 4096)
	if err != nil {
		return 0, err
	}
	end := strings.LastIndex(string(raw), ") ")
	if !bytes.HasPrefix(raw, []byte("1 (")) || end < 3 {
		return 0, ErrBinding
	}
	fields := strings.Fields(string(raw[end+2:]))
	if len(fields) < 20 {
		return 0, ErrBinding
	}
	started, err = strconv.ParseUint(fields[19], 10, 64)
	if err != nil || started == 0 || strconv.FormatUint(started, 10) != fields[19] {
		return 0, ErrBinding
	}
	return started, nil
}

func (i *runtimeIdentity) check() (err error) {
	if i == nil || i.ctx == nil || i.closed {
		return ErrInput
	}
	if i.terminal != nil {
		return i.terminal
	}
	defer func() {
		err = errors.Join(err, i.ctx.Err())
		if err != nil {
			i.terminal = err
		}
	}()
	if err = i.ctx.Err(); err != nil {
		return err
	}
	for name, file := range i.namespaces {
		opened, e := file.Stat()
		named, ne := os.Stat("/proc/self/ns/" + name) // intentional original kernel namespace link
		var fs syscall.Statfs_t
		fe := syscall.Fstatfs(int(file.Fd()), &fs)
		if e != nil || ne != nil || fe != nil || fs.Type != 0x6e736673 || !os.SameFile(i.identities[name], opened) || !os.SameFile(opened, named) {
			return errors.Join(ErrBinding, e, ne, fe)
		}
		if i.requireManagerNamespaces {
			managerNamespace, err := os.Stat("/proc/1/ns/" + name)
			if err != nil || !os.SameFile(opened, managerNamespace) {
				return errors.Join(ErrBinding, err)
			}
		}
	}
	manager, e := i.manager.Stat()
	named, ne := os.Lstat("/proc/1")
	var fs syscall.Statfs_t
	fe := syscall.Fstatfs(int(i.manager.Fd()), &fs)
	if e != nil || ne != nil || fe != nil || fs.Type != 0x9fa0 || !manager.IsDir() || !os.SameFile(i.managerInfo, manager) || !os.SameFile(manager, named) {
		return errors.Join(ErrBinding, e, ne, fe)
	}
	started, e := i.managerStart()
	if e != nil && started == 0 {
		return e
	}
	// A nonzero clock was parsed before a possible cancellation or Close
	// failure. Retain an observed foreign clock alongside that actual error.
	if started != i.started {
		return errors.Join(ErrBinding, e)
	}
	if e != nil {
		return e
	}
	boot, e := i.boot.Stat()
	bootPath, be := os.Lstat("/proc/sys/kernel/random/boot_id")
	if e != nil || be != nil || !os.SameFile(i.bootInfo, boot) || !os.SameFile(boot, bootPath) {
		return errors.Join(ErrBinding, e, be)
	}
	raw, e := readKernelFile(i.boot, 128)
	if e != nil || !bytes.Equal(raw, i.bootRaw) {
		return errors.Join(ErrBinding, e)
	}
	return nil
}

func (i *runtimeIdentity) close() error {
	if i == nil {
		return nil
	}
	if !i.closed {
		i.closed = true
		for _, file := range i.namespaces {
			i.terminal = errors.Join(i.terminal, file.Close())
		}
		if i.boot != nil {
			i.terminal = errors.Join(i.terminal, i.boot.Close())
		}
		if i.manager != nil {
			i.terminal = errors.Join(i.terminal, i.manager.Close())
		}
		i.terminal = errors.Join(i.terminal, i.ctx.Err())
	}
	return i.terminal
}
