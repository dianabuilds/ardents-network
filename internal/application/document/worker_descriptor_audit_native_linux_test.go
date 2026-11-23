//go:build linux && document_worker_installed

package document

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// These inherited-descriptor audit tests exercise the fixed installed worker's
// native admission: they replace the process stdio with the worker's inherited
// shape and open real cgroup control files, so they require the native lane
// (a real unified cgroup and a clean descriptor table), not the ordinary
// shuffled suite running in an arbitrary container. The pure path/name policy is
// covered separately by TestManagerCgroupControlTargetPolicy.

// withWorkerStdio replaces the process stdio with the fixed worker's inherited
// shape — one AF_UNIX socket for input and output, null diagnostics — for the
// duration of fn, then restores the original descriptors. Saved descriptors
// carry close-on-exec so the audit does not observe the test's own stdio.
func withWorkerStdio(t *testing.T, fn func()) {
	t.Helper()
	pair, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := setCloseOnExec(pair[1]); err != nil {
		t.Fatal(err)
	}
	null, err := syscall.Open("/dev/null", syscall.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	saved := [3]int{}
	for fd := range saved {
		duplicate, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_DUPFD_CLOEXEC, 0)
		if errno != 0 {
			t.Fatal(errno)
		}
		saved[fd] = int(duplicate)
	}
	restore := func() {
		for fd, original := range saved {
			if err := syscall.Dup3(original, fd, 0); err != nil {
				t.Error(err)
			}
			_ = syscall.Close(original)
		}
		_ = syscall.Close(pair[0])
		_ = syscall.Close(pair[1])
		_ = syscall.Close(null)
	}
	defer restore()
	if err := syscall.Dup3(pair[0], 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Dup3(pair[0], 1, 0); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Dup3(null, 2, 0); err != nil {
		t.Fatal(err)
	}
	// The original socket and null descriptors stay open for restoration; they
	// must not appear as inherited authority during the audit.
	if err := setCloseOnExec(pair[0]); err != nil {
		t.Fatal(err)
	}
	if err := setCloseOnExec(null); err != nil {
		t.Fatal(err)
	}
	closeStrayDescriptors(t)
	fn()
}

func setCloseOnExec(fd int) error {
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_SETFD, syscall.FD_CLOEXEC)
	if errno != 0 {
		return errno
	}
	return nil
}

// closeStrayDescriptors removes non-close-on-exec descriptors above stdio that
// earlier tests may have left in the shared test process. The real fixed worker
// starts with a clean descriptor table, so the audit tests must not observe
// another test's leaked descriptor under an arbitrary shuffle order. Runtime and
// test-framework descriptors carry close-on-exec and are untouched; the test's
// own inherited descriptor is created after this cleanup.
func closeStrayDescriptors(t *testing.T) {
	t.Helper()
	dir, err := syscall.Open("/proc/self/fd", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(dir)
	var buf [4096]byte
	for {
		n, err := syscall.ReadDirent(dir, buf[:])
		if err != nil || n == 0 {
			return
		}
		_, _, entries := syscall.ParseDirent(buf[:n], -1, nil)
		for _, entry := range entries {
			fd, err := strconv.Atoi(entry)
			if err != nil || fd <= 2 || fd == dir {
				continue
			}
			flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFD, 0)
			if errno == 0 && flags&syscall.FD_CLOEXEC == 0 {
				_ = syscall.Close(fd)
			}
		}
	}
}

// A resource-limited service on systemd 255 inherits its own cgroup cpu.max
// without close-on-exec. The audit must verify the descriptor against the
// kernel-resolved own-cgroup path, close it (the worker never manages its own
// cgroup) and admit the worker; no control authority survives admission.
func TestVerifyWorkerDescriptorsClosesManagerCgroupControl(t *testing.T) {
	ownPath, err := workerUnifiedCgroupPath()
	if err != nil {
		t.Fatal("invalid environment: unified cgroup placement unavailable:", err)
	}
	withWorkerStdio(t, func() {
		control, err := syscall.Open("/sys/fs/cgroup"+ownPath+"/cpu.max", syscall.O_RDONLY, 0)
		if err != nil {
			t.Fatal("invalid environment: own cgroup cpu.max unavailable:", err)
		}
		if err := verifyWorkerDescriptors(); err != nil {
			t.Fatal("manager-passed own-cgroup control was refused:", err)
		}
		if _, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(control), syscall.F_GETFD, 0); errno == 0 {
			_ = syscall.Close(control)
			t.Fatal("manager control descriptor survived admission")
		}
	})
}

// Foreign authority remains refused even when it lives in the worker's own
// cgroup directory: the migration surface cgroup.procs, a foreign regular
// file, a foreign local socket and the cgroup directory itself are not
// resource controls and must not reach the worker exchange.
func TestVerifyWorkerDescriptorsRefusesForeignAuthority(t *testing.T) {
	ownPath, err := workerUnifiedCgroupPath()
	if err != nil {
		t.Fatal("invalid environment: unified cgroup placement unavailable:", err)
	}
	foreignPath := filepath.Join(t.TempDir(), "foreign")
	if err := os.WriteFile(foreignPath, []byte("foreign authority"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		open func() (int, error)
	}{
		{"own cgroup migration surface", func() (int, error) {
			return syscall.Open("/sys/fs/cgroup"+ownPath+"/cgroup.procs", syscall.O_RDONLY, 0)
		}},
		{"own cgroup directory", func() (int, error) {
			return syscall.Open("/sys/fs/cgroup"+ownPath, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
		}},
		{"foreign regular file", func() (int, error) {
			return syscall.Open(foreignPath, syscall.O_RDONLY, 0)
		}},
		{"foreign local socket", func() (int, error) {
			pair, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
			if err != nil {
				return -1, err
			}
			_ = syscall.Close(pair[1])
			return pair[0], nil
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			withWorkerStdio(t, func() {
				fd, err := test.open()
				if err != nil {
					t.Fatal(err)
				}
				defer syscall.Close(fd)
				err = verifyWorkerDescriptors()
				if err == nil || !strings.Contains(err.Error(), "foreign descriptor") {
					t.Fatal("foreign authority was not refused:", err)
				}
			})
		})
	}
}

// A foreign descriptor that carries close-on-exec did not survive exec and is
// not inherited authority; the audit ignores it exactly as before.
func TestVerifyWorkerDescriptorsIgnoresCloseOnExecForeign(t *testing.T) {
	foreignPath := filepath.Join(t.TempDir(), "foreign")
	if err := os.WriteFile(foreignPath, []byte("post-exec descriptor"), 0600); err != nil {
		t.Fatal(err)
	}
	withWorkerStdio(t, func() {
		fd, err := syscall.Open(foreignPath, syscall.O_RDONLY|syscall.O_CLOEXEC, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer syscall.Close(fd)
		if err := verifyWorkerDescriptors(); err != nil {
			t.Fatal("close-on-exec descriptor was treated as inherited:", err)
		}
	})
}
