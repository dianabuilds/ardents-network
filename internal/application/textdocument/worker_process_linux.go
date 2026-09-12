//go:build linux

package textdocument

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
)

// RunInheritedWorker admits only the installed entrypoint's AF_UNIX stdio
// attachment and null stderr. This descriptor check does not attest installed
// confinement: Endpoint independently verifies the unit/root/accepted socket.
func RunInheritedWorker(mode WorkerMode) error {
	if mode != ReaderWorker && mode != PublisherWorker {
		return errors.New("text worker mode is unavailable")
	}
	if err := verifyWorkerDescriptors(); err != nil {
		return err
	}
	// Create the pollable duplicate only after auditing inherited authority.
	// Closing this connection interrupts blocked I/O so cancellation can join.
	connection, err := net.FileConn(os.Stdin)
	if err != nil {
		return errors.New("text worker attachment cannot be polled")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	return RunWorker(ctx, &inheritedWorkerAttachment{Conn: connection}, mode)
}

func verifyWorkerDescriptors() error {
	input, err := os.Stdin.Stat()
	if err != nil || input.Mode()&os.ModeSocket == 0 {
		return errors.New("text worker input attachment is invalid")
	}
	output, err := os.Stdout.Stat()
	if err != nil || !os.SameFile(input, output) {
		return errors.New("text worker output attachment is invalid")
	}
	address, err := syscall.Getsockname(0)
	if _, ok := address.(*syscall.SockaddrUnix); err != nil || !ok {
		return errors.New("text worker attachment is not local")
	}
	stderr, err := os.Stderr.Stat()
	if err != nil {
		return errors.New("text worker diagnostics attachment is invalid")
	}
	null, err := os.Stat("/dev/null")
	if err != nil || stderr.Mode()&os.ModeCharDevice == 0 || null.Mode()&os.ModeCharDevice == 0 {
		return errors.New("text worker diagnostics attachment is invalid")
	}
	// PrivateDevices creates a private /dev/null inode; the inherited null
	// stderr may name the host inode. The kernel device identity must match.
	stderrStat, stderrOK := stderr.Sys().(*syscall.Stat_t)
	nullStat, nullOK := null.Sys().(*syscall.Stat_t)
	if !stderrOK || !nullOK || stderrStat.Rdev != nullStat.Rdev {
		return errors.New("text worker diagnostics attachment is invalid")
	}
	// Go's runtime may initialize epoll/eventfd descriptors before ReadDir.
	// FD_CLOEXEC distinguishes those post-exec descriptors from authority that
	// actually survived into this executable.
	directory, err := syscall.Open("/proc/self/fd", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return errors.New("text worker descriptor inventory is unavailable")
	}
	defer syscall.Close(directory)
	var buffer [4096]byte
	for {
		n, err := syscall.ReadDirent(directory, buffer[:])
		if err != nil {
			return errors.New("text worker descriptor inventory is unavailable")
		}
		if n == 0 {
			return nil
		}
		_, _, entries := syscall.ParseDirent(buffer[:n], -1, nil)
		for _, entry := range entries {
			fd, err := strconv.Atoi(entry)
			if err != nil {
				return errors.New("text worker inherited a foreign descriptor")
			}
			if fd <= 2 || fd == directory {
				continue
			}
			survived, err := descriptorSurvivedExec(fd)
			if err != nil {
				return errors.New("text worker descriptor inventory is unavailable")
			}
			if survived {
				var target [256]byte
				n, targetErr := syscall.Readlink("/proc/self/fd/"+entry, target[:])
				if targetErr != nil {
					return errors.New("text worker descriptor inventory is unavailable")
				}
				return fmt.Errorf("text worker inherited a foreign descriptor fd=%d target=%q", fd, target[:n])
			}
		}
	}
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
