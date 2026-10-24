package infrastructure

import (
	"context"
	"errors"
	"os"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// This scope is used only by the dedicated canonical transition controller.
// Its caller locks the launching OS thread until join (Pdeathsig is bound to
// that thread). The controller has no other child processes. It remains a
// subreaper until its canonical process exits; it is not a reusable executor.
// There is deliberately no new PID/mount/cgroup namespace: the coordinator
// must still observe the actual original service manager namespaces.
type sourceTransitionScope struct {
	ioMu        sync.Mutex
	pid         int
	pidfd       *os.File
	process     *os.File
	processInfo os.FileInfo
	birth       sourceTransitionProcess
	denied      bool
	joined      bool
	status      syscall.WaitStatus
	reaped      int
	terminal    error
	// Failure-only native barrier after actual stop/readback registration.
	// It can hold or cancel, never supply a successful observation.
	frozen func()
}

func forkSourceTransition(program, endpoint *os.File, argv, environment []string) (*sourceTransitionScope, error) {
	if program == nil || endpoint == nil || len(argv) == 0 || os.Geteuid() != 0 || os.Getegid() != 0 {
		return nil, ErrInput
	}
	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0); err != nil {
		return nil, err
	}
	pidfd := -1
	pid, err := syscall.ForkExec("/proc/self/fd/3", argv, &syscall.ProcAttr{
		Env: environment, Files: []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd(), program.Fd(), endpoint.Fd()},
		Sys: &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL, PidFD: &pidfd},
	})
	if err != nil {
		return nil, err
	}
	s := &sourceTransitionScope{pid: pid}
	if pidfd < 0 {
		// An older kernel may launch without the requested handle. No operation
		// is admitted; retain the unreaped group leader while denying/joining.
		s.terminal = ErrBinding
		return nil, errors.Join(ErrBinding, s.join())
	}
	s.pidfd = os.NewFile(uintptr(pidfd), "source-transition-original-pidfd")
	if err := s.retainProcess(); err != nil {
		return nil, errors.Join(err, s.join())
	}
	return s, nil
}

// waitStopped observes without reaping, including when exit races the stop.
// The unreaped original group leader prevents numeric process-group reuse.
// A packet alone never establishes a stopped boundary.
func (s *sourceTransitionScope) waitStopped(ctx context.Context) error {
	if s == nil || ctx == nil {
		return ErrInput
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.ioMu.Lock()
		if s.denied || s.joined || s.pidfd == nil {
			s.ioMu.Unlock()
			return ErrInput
		}
		var info unix.Siginfo
		err := unix.Waitid(unix.P_PIDFD, int(s.pidfd.Fd()), &info, unix.WSTOPPED|unix.WNOHANG|unix.WNOWAIT, nil)
		var ready int
		if err == nil {
			ready, err = unix.Poll([]unix.PollFd{{Fd: int32(s.pidfd.Fd()), Events: unix.POLLIN}}, 0)
		}
		s.ioMu.Unlock()
		// A signal can interrupt either native observation without changing
		// child state. Reobserve under the same deadline and original pidfd.
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		if ready != 0 {
			return ErrSourceRepairRequired
		}
		// Linux CLD_STOPPED is 5. Siginfo's union stays opaque: no unsafe or
		// architecture-dependent reads are needed with the original pidfd.
		if info.Signo == int32(syscall.SIGCHLD) && info.Code == 5 {
			return nil
		}
		if info.Signo != 0 {
			return ErrBinding
		}
		if err = waitSourceTransitionTick(ctx); err != nil {
			return err
		}
	}
}

func waitSourceTransitionTick(ctx context.Context) error {
	timer := time.NewTimer(5 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *sourceTransitionScope) resume() error {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	if s.denied || s.joined || s.pidfd == nil {
		return ErrInput
	}
	return unix.PidfdSendSignal(int(s.pidfd.Fd()), unix.SIGCONT, nil, 0)
}

func (s *sourceTransitionScope) awaitExit(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.ioMu.Lock()
		if s.denied || s.joined || s.pidfd == nil {
			s.ioMu.Unlock()
			return ErrInput
		}
		n, err := unix.Poll([]unix.PollFd{{Fd: int32(s.pidfd.Fd()), Events: unix.POLLIN}}, 0)
		s.ioMu.Unlock()
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		if n != 0 {
			return nil
		}
		if err = waitSourceTransitionTick(ctx); err != nil {
			return err
		}
	}
}

func (s *sourceTransitionScope) deny() error {
	if s == nil {
		return nil
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	return s.denyLocked()
}

func (s *sourceTransitionScope) denyLocked() error {
	if s.joined {
		return s.terminal
	}
	s.denied = true
	// No Wait call has reaped the original leader. Kill its exact group before
	// any reap, then signal the from-birth pidfd as a separate original handle.
	groupErr := syscall.Kill(-s.pid, syscall.SIGKILL)
	if errors.Is(groupErr, syscall.ESRCH) {
		groupErr = nil
	}
	var originalErr error
	if s.pidfd != nil {
		originalErr = unix.PidfdSendSignal(int(s.pidfd.Fd()), unix.SIGKILL, nil, 0)
		if errors.Is(originalErr, syscall.ESRCH) {
			originalErr = nil
		}
	}
	s.terminal = errors.Join(s.terminal, groupErr, originalErr)
	return s.terminal
}

// join is physical and idempotent. A deadline never substitutes for the real
// Wait. The dedicated subreaper consumes all of its own adopted descendants,
// through ECHILD, rather than inferring whole-scope completion from one PID.
func (s *sourceTransitionScope) join() error {
	if s == nil {
		return nil
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	if s.joined {
		return s.terminal
	}
	s.denyLocked()
	leader := false
	for {
		var status syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &status, 0, nil)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if errors.Is(err, syscall.ECHILD) {
			break
		}
		if err != nil {
			s.terminal = errors.Join(s.terminal, err)
			break
		}
		if pid == s.pid {
			s.status = status
			leader = true
		}
		s.reaped++
	}
	if !leader {
		s.terminal = errors.Join(s.terminal, ErrBinding)
	}
	if s.pidfd != nil {
		s.terminal = errors.Join(s.terminal, s.pidfd.Close())
	}
	if s.process != nil {
		s.terminal = errors.Join(s.terminal, s.process.Close())
	}
	s.joined = true
	return s.terminal
}
