package process

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
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
	if err = pin.retainSourceNamespaces(ctx); err == nil {
		err = pin.Observe(ctx)
	}
	if err != nil {
		return nil, errors.Join(err, pin.Close())
	}
	return pin, nil
}

// Namespace links are intentional kernel links, opened only for the original
// already matched process. Observe checks the retained proc/start clock around
// them, so PID reuse cannot select another namespace. Endpoint policy is unchanged.
func (pin *Invocation) retainSourceNamespaces(ctx context.Context) error {
	pin.namespaces = make(map[string]*os.File)
	for _, name := range []string{"mnt", "pid", "cgroup", "uts", "user", "net", "ipc"} {
		if err := ctx.Err(); err != nil {
			return err
		}
		file, err := os.OpenFile(filepath.Join("/proc", strconv.FormatUint(uint64(pin.pid), 10), "ns", name), os.O_RDONLY|syscall.O_CLOEXEC, 0)
		if err != nil {
			return err
		}
		pin.namespaces[name] = file
	}
	return pin.observeSourceNamespaces(ctx)
}

func (pin *Invocation) observeSourceNamespaces(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for name, file := range pin.namespaces {
		opened, err := file.Stat()
		named, pathErr := os.Stat(filepath.Join("/proc", strconv.FormatUint(uint64(pin.pid), 10), "ns", name))
		var fs syscall.Statfs_t
		fsErr := syscall.Fstatfs(int(file.Fd()), &fs)
		if err != nil || pathErr != nil || fsErr != nil || fs.Type != 0x6e736673 || !os.SameFile(opened, named) {
			return errors.Join(errBinding, err, pathErr, fsErr)
		}
	}
	return ctx.Err()
}

func sourceInvocationPolicy(expected SourceExpected) invocationPolicy {
	program := filepath.Join(expected.InstallationRoot, "generations", expected.Generation, "ardents-node-linux-amd64")
	return invocationPolicy{program: program,
		arguments: strings.Join([]string{program, "source", "start-installed", filepath.Join(expected.InstallationRoot, "binding.json"), ""}, "\x00"),
		placement: "0::/system.slice/ardents-source.service\n"}
}
