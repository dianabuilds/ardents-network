package process

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// SourceExpected contains already checked Source installation facts. It grants
// no Release or startup right. Its fixed policy differs from Endpoint's policy:
// one node executable, Source argv and Source unit, without a configurable selector.
type SourceExpected struct {
	PID, UID, GID                uint32
	Invocation                   [16]byte
	InstallationRoot, Generation string
	ProgramIdentity              os.FileInfo
}

// RetainSource independently retains one actual process for root's original
// Source start attempt. Its caller owns admission, stop and original cgroup join.
func RetainSource(ctx context.Context, expected SourceExpected) (*Invocation, error) {
	if os.Geteuid() != 0 {
		return nil, errInput
	}
	return retainSource(ctx, expected)
}

// RetainSourceStop observes the exact already archived original process before
// explicit-stop admission. Detached inode facts grant no right; this opener
// still checks the genuine process, executable, argv, namespace and pidfd.
func RetainSourceStop(ctx context.Context, expected SourceExpected, device, inode uint64) (*Invocation, error) {
	if device == 0 || inode == 0 {
		return nil, errInput
	}
	pin, err := RetainSource(ctx, expected)
	if err != nil {
		return nil, err
	}
	native, ok := pin.identity.Sys().(*syscall.Stat_t)
	if !ok || uint64(native.Dev) != device || native.Ino != inode {
		return nil, errors.Join(errBinding, pin.Close())
	}
	return pin, nil
}

// RetainSourceSelf can observe only the actual non-root Source calling process.
// It cannot borrow root's observation or select another PID or executable.
func RetainSourceSelf(ctx context.Context, expected SourceExpected) (*Invocation, error) {
	if os.Geteuid() == 0 || expected.PID != uint32(os.Getpid()) ||
		expected.UID != uint32(os.Geteuid()) || expected.GID != uint32(os.Getegid()) {
		return nil, errInput
	}
	return retainSource(ctx, expected)
}

func retainSource(ctx context.Context, expected SourceExpected) (*Invocation, error) {
	if expected.UID < 100 || expected.UID > 999 || expected.GID != expected.UID {
		return nil, errInput
	}
	facts := Expected{PID: expected.PID, UID: expected.UID, GID: expected.GID,
		Invocation: expected.Invocation, InstallationRoot: expected.InstallationRoot,
		Generation: expected.Generation, ProgramIdentity: expected.ProgramIdentity}
	pin, err := retainFixed(ctx, facts, sourceInvocationPolicy(expected))
	if err != nil {
		return nil, err
	}
	if err = pin.retainNamespaces(ctx); err == nil {
		err = pin.Observe(ctx)
	}
	if err == nil && os.Geteuid() == 0 {
		err = pin.retainSourcePIDFD(ctx)
	}
	if err != nil {
		return nil, errors.Join(err, pin.Close())
	}
	return pin, nil
}

// The installed Source contract selects Linux amd64. These kernel entrypoints
// have no pointer arguments, cgo or unsafe. Pin before reobserving original proc
// identity/start clock, so PID reuse cannot turn a signal into a foreign effect.
type sourceSignal struct {
	file        *os.File
	interrupted bool
}

func (pin *Invocation) retainSourcePIDFD(ctx context.Context) error {
	if runtime.GOARCH != "amd64" || pin == nil || pin.sourceSignal != nil {
		return errInput
	}
	const pidfdOpenAMD64 = 434
	fd, _, failure := syscall.Syscall(pidfdOpenAMD64, uintptr(pin.pid), 0, 0)
	if failure != 0 {
		return failure
	}
	pin.sourceSignal = &sourceSignal{file: os.NewFile(fd, "original Source pidfd")}
	return pin.Observe(ctx)
}

// InterruptSource sends one SIGTERM through the retained original pidfd. It is
// a physical operation only; Installation separately admits explicit stop and
// retains the complete domain result and original cgroup join. Its caller
// serializes this operation and joins it before Close; no numeric PID kill,
// reconnect, replacement process or accepting authority is supplied here.
func (pin *Invocation) InterruptSource(ctx context.Context) (bool, error) {
	if pin == nil || ctx == nil || os.Geteuid() != 0 || runtime.GOARCH != "amd64" || pin.sourceSignal == nil || pin.sourceSignal.file == nil || pin.sourceSignal.interrupted {
		return false, errInput
	}
	if err := pin.Observe(ctx); err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	pin.sourceSignal.interrupted = true
	const pidfdSendSignalAMD64 = 424
	_, _, failure := syscall.Syscall6(pidfdSendSignalAMD64, pin.sourceSignal.file.Fd(), uintptr(syscall.SIGTERM), 0, 0, 0, 0)
	if failure != 0 {
		return true, failure
	}
	return true, ctx.Err()
}

func sourceInvocationPolicy(expected SourceExpected) invocationPolicy {
	program := filepath.Join(expected.InstallationRoot, "generations", expected.Generation, "ardents-node-linux-amd64")
	return invocationPolicy{program: program,
		arguments: strings.Join([]string{program, "source", "start-installed", filepath.Join(expected.InstallationRoot, "binding.json"), ""}, "\x00"),
		placement: "0::/system.slice/ardents-source.service\n"}
}
