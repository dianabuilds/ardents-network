//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/dianabuilds/ardents-network/internal/application/document"
	"io"
	"net"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// runInheritedWorker admits only the installed entrypoint's AF_UNIX stdio
// attachment and null stderr, plus any resource-control descriptor the system
// manager passed for the worker's OWN cgroup (systemd 255 hands cpu.max to a
// resource-limited service without close-on-exec). Each such descriptor is
// identity-verified against the kernel-resolved own-cgroup path and closed
// before admission completes: the fixed worker never manages its own cgroup,
// so no inherited authority survives the audit. This descriptor check does not
// attest installed confinement: Endpoint independently verifies the unit/root/
// accepted socket.
func runInheritedWorker(mode document.WorkerMode) error {
	if mode != document.ReaderWorker && mode != document.PublisherWorker {
		return errors.New("document worker mode is unavailable")
	}
	if err := verifyWorkerDescriptors(); err != nil {
		return err
	}
	// Create the pollable duplicate only after auditing inherited authority.
	// Closing this connection interrupts blocked I/O so cancellation can join.
	connection, err := net.FileConn(os.Stdin)
	if err != nil {
		return errors.New("document worker attachment cannot be polled")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	return document.RunWorker(ctx, &inheritedWorkerAttachment{Conn: connection}, mode)
}

func verifyWorkerDescriptors() error {
	input, err := os.Stdin.Stat()
	if err != nil || input.Mode()&os.ModeSocket == 0 {
		return errors.New("document worker input attachment is invalid")
	}
	output, err := os.Stdout.Stat()
	if err != nil || !os.SameFile(input, output) {
		return errors.New("document worker output attachment is invalid")
	}
	address, err := syscall.Getsockname(0)
	if _, ok := address.(*syscall.SockaddrUnix); err != nil || !ok {
		return errors.New("document worker attachment is not local")
	}
	stderr, err := os.Stderr.Stat()
	if err != nil {
		return errors.New("document worker diagnostics attachment is invalid")
	}
	null, err := os.Stat("/dev/null")
	if err != nil || stderr.Mode()&os.ModeCharDevice == 0 || null.Mode()&os.ModeCharDevice == 0 {
		return errors.New("document worker diagnostics attachment is invalid")
	}
	// PrivateDevices creates a private /dev/null inode; the inherited null
	// stderr may name the host inode. The kernel device identity must match.
	stderrStat, stderrOK := stderr.Sys().(*syscall.Stat_t)
	nullStat, nullOK := null.Sys().(*syscall.Stat_t)
	if !stderrOK || !nullOK || stderrStat.Rdev != nullStat.Rdev {
		return errors.New("document worker diagnostics attachment is invalid")
	}
	// Go's runtime may initialize epoll/eventfd descriptors before ReadDir.
	// FD_CLOEXEC distinguishes those post-exec descriptors from authority that
	// actually survived into this executable.
	directory, err := syscall.Open("/proc/self/fd", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return errors.New("document worker descriptor inventory is unavailable")
	}
	defer syscall.Close(directory)
	// Resolved lazily: a manager that passes no control descriptor (systemd 249)
	// never requires the observation, and its behavior is unchanged.
	var ownCgroupPath string
	var buffer [4096]byte
	for {
		n, err := syscall.ReadDirent(directory, buffer[:])
		if err != nil {
			return errors.New("document worker descriptor inventory is unavailable")
		}
		if n == 0 {
			return nil
		}
		_, _, entries := syscall.ParseDirent(buffer[:n], -1, nil)
		for _, entry := range entries {
			fd, err := strconv.Atoi(entry)
			if err != nil {
				return errors.New("document worker inherited a foreign descriptor")
			}
			if fd <= 2 || fd == directory {
				continue
			}
			survived, err := descriptorSurvivedExec(fd)
			if err != nil {
				return errors.New("document worker descriptor inventory is unavailable")
			}
			if survived {
				if ownCgroupPath == "" {
					path, pathErr := workerUnifiedCgroupPath()
					if pathErr != nil {
						return errors.New("document worker inherited a foreign descriptor")
					}
					ownCgroupPath = path
				}
				if !managerCgroupControlFD(fd, ownCgroupPath) {
					return errors.New("document worker inherited a foreign descriptor")
				}
				if err := syscall.Close(fd); err != nil {
					return errors.New("document worker descriptor inventory is unavailable")
				}
			}
		}
	}
}

// managerCgroupControlNames are the cgroup v2 resource-control files a system
// manager may hand to its own resource-limited service without close-on-exec.
// Each grants only the service's own limit management, bounded by its parent
// cgroup. Migration, controller-enablement and lifecycle surfaces
// (cgroup.procs, cgroup.threads, cgroup.subtree_control, cgroup.type,
// cgroup.kill, cgroup.freeze, ...) are deliberately absent: they are not
// resource controls and remain foreign authority.
var managerCgroupControlNames = map[string]bool{
	"cpu.max": true, "cpu.weight": true, "cpu.weight.nice": true, "cpu.idle": true,
	"memory.min": true, "memory.low": true, "memory.high": true, "memory.max": true,
	"memory.swap.max": true, "memory.zswap.max": true,
	"pids.max": true, "io.max": true, "io.weight": true,
}

// managerCgroupControlTarget admits only a control file directly inside the
// worker's own unified cgroup. Pure path/name policy; the caller supplies the
// kernel-resolved descriptor target and independently verifies the file kind.
func managerCgroupControlTarget(target, ownCgroupPath string) bool {
	name, ok := strings.CutPrefix(target, path.Join("/sys/fs/cgroup", ownCgroupPath)+"/")
	return ok && managerCgroupControlNames[name]
}

// managerCgroupControlFD verifies one surviving descriptor is a regular
// resource-control file of the worker's own cgroup. The readlink target is the
// kernel-resolved path (symlinks and hard links cannot disguise a foreign
// file); the caller closes the descriptor after admission.
func managerCgroupControlFD(fd int, ownCgroupPath string) bool {
	target, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", fd))
	if err != nil || !managerCgroupControlTarget(target, ownCgroupPath) {
		return false
	}
	var status syscall.Stat_t
	if err := syscall.Fstat(fd, &status); err != nil || status.Mode&syscall.S_IFMT != syscall.S_IFREG {
		return false
	}
	return true
}

// workerUnifiedCgroupPath reads the kernel-provided unified placement. The
// input is trusted, but a missing, relative or traversing path refuses
// admission outright rather than guessing a cgroup identity.
func workerUnifiedCgroupPath() (string, error) {
	raw, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		path, ok := strings.CutPrefix(line, "0::")
		if !ok {
			continue
		}
		if !strings.HasPrefix(path, "/") || strings.Contains(path, "..") {
			return "", errors.New("document worker unified cgroup placement is invalid")
		}
		return path, nil
	}
	return "", errors.New("document worker unified cgroup placement is unavailable")
}

func descriptorSurvivedExec(fd int) (bool, error) {
	flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFD, 0)
	if errno != 0 {
		return false, errno
	}
	return flags&syscall.FD_CLOEXEC == 0, nil
}

type inheritedWorkerAttachment struct {
	net.Conn
	once sync.Once
	err  error
}

func (attachment *inheritedWorkerAttachment) Close() error {
	attachment.once.Do(func() { attachment.err = errors.Join(attachment.Conn.Close(), os.Stdin.Close(), os.Stdout.Close()) })
	return attachment.err
}

var _ io.ReadWriteCloser = (*inheritedWorkerAttachment)(nil)
