package process

import (
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"syscall"
)

// CheckSourceRecoveryQuiescent refuses any other process of the actual fixed
// service principal, including invocations outside the platform unit. The
// caller holds both semantic leases and independently checks the fixed manager
// and cgroup. Those leases fence new invocations; this scan excludes every old
// process which could still own a volatile completion and later remove a new
// producer. It grants no joined-completion, signal or process-adoption right.
func CheckSourceRecoveryQuiescent(ctx context.Context) (result error) {
	if ctx == nil || os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getuid() != os.Geteuid() || os.Getegid() != os.Geteuid() {
		return errInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := os.OpenFile("/proc", os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, root.Close(), ctx.Err()) }()
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(root.Fd()), &fs); err != nil || fs.Type != 0x9fa0 {
		return errors.Join(errBinding, err)
	}
	if err := checkSourceRecoveryProcMount(root); err != nil {
		return err
	}
	self := strconv.Itoa(os.Getpid())
	count := 0
	for {
		names, readErr := root.Readdirnames(128)
		if readErr != nil && readErr != io.EOF {
			return readErr
		}
		count += len(names)
		if count > 65536 {
			return errBinding
		}
		for _, name := range names {
			if err := ctx.Err(); err != nil {
				return err
			}
			pid, err := strconv.ParseUint(name, 10, 32)
			if err != nil || pid == 0 || name == self {
				continue
			}
			if strconv.FormatUint(pid, 10) != name {
				return errBinding
			}
			if err := checkSourceRecoveryPID(root, name, uint32(os.Geteuid())); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			return nil
		}
	}
}

func checkSourceRecoveryProcMount(root *os.File) (result error) {
	fd, err := syscall.Openat(int(root.Fd()), "self/mountinfo", syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), "Source recovery proc mount")
	defer func() { result = errors.Join(result, file.Close()) }()
	raw, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 {
		return errors.Join(errBinding, err)
	}
	return sourceRecoveryProcMount(raw)
}

func checkSourceRecoveryPID(root *os.File, name string, uid uint32) (result error) {
	fd, err := syscall.Openat(int(root.Fd()), name+"/status", syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if errors.Is(err, syscall.ENOENT) {
		return nil
	} // Gone, never joined success.
	if err != nil {
		return err
	}
	status := os.NewFile(uintptr(fd), "Source recovery process status")
	defer func() { result = errors.Join(result, status.Close()) }()
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(fd, &fs); err != nil || fs.Type != 0x9fa0 {
		return errors.Join(errBinding, err)
	}
	raw, err := io.ReadAll(io.LimitReader(status, 65537))
	if err != nil || len(raw) > 65536 {
		return errors.Join(errBinding, err)
	}
	retained, err := sourceRecoveryPrincipal(raw, uid)
	if err != nil || retained {
		return errors.Join(errBinding, err)
	}
	return nil
}
