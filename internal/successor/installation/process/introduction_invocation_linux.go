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

// IntroductionExpected contains independently checked installation/manager facts,
// never an accepting startup proof. No input chooses another Node duty or unit.
type IntroductionExpected struct {
	PID, UID, GID                uint32
	Invocation                   [16]byte
	InstallationRoot, Generation string
	ProgramIdentity              os.FileInfo
}

// IntroductionInvocation retains its own original kernel observations. Copies
// share admission, the first failure and physically joined Close. It exposes
// neither the underlying Invocation nor Source's original signal right.
type IntroductionInvocation struct{ original *nodeInvocation }

// RetainIntroduction independently observes root's exact original Introduction
// process. Installation separately admits start/stop and retains original cgroup observation.
func RetainIntroduction(ctx context.Context, expected IntroductionExpected) (IntroductionInvocation, error) {
	if os.Geteuid() != 0 {
		return IntroductionInvocation{}, errInput
	}
	return retainIntroduction(ctx, expected)
}

// RetainIntroductionSelf observes only this actual non-root calling process.
// Root's observation cannot be passed in as a substitute for this acquisition.
func RetainIntroductionSelf(ctx context.Context, expected IntroductionExpected) (IntroductionInvocation, error) {
	if os.Geteuid() == 0 || expected.PID != uint32(os.Getpid()) ||
		expected.UID != uint32(os.Geteuid()) || expected.GID != uint32(os.Getegid()) {
		return IntroductionInvocation{}, errInput
	}
	return retainIntroduction(ctx, expected)
}

func retainIntroduction(ctx context.Context, expected IntroductionExpected) (IntroductionInvocation, error) {
	if runtime.GOARCH != "amd64" || expected.UID < 100 || expected.UID > 999 || expected.GID != expected.UID {
		return IntroductionInvocation{}, errInput
	}
	facts := Expected(expected)
	pin, err := retainFixed(ctx, facts, introductionInvocationPolicy(expected))
	if err != nil {
		return IntroductionInvocation{}, err
	}
	o := &nodeInvocation{pin: pin}
	result := IntroductionInvocation{original: o}
	if err = pin.retainNamespaces(ctx); err == nil {
		err = pin.Observe(ctx)
	}
	if err == nil && os.Geteuid() == 0 {
		// Linux amd64, no pointer arguments, cgo or unsafe. Reobserve after
		// opening: a reused numeric PID cannot authorize a foreign signal.
		const pidfdOpenAMD64 = 434
		fd, _, failure := syscall.Syscall(pidfdOpenAMD64, uintptr(expected.PID), 0, 0)
		if failure != 0 {
			err = failure
		} else {
			o.pidfd = os.NewFile(fd, "original Introduction pidfd")
			err = pin.Observe(ctx)
		}
	}
	if err != nil {
		o.failure = err
		return IntroductionInvocation{}, errors.Join(err, result.Close())
	}
	return result, nil
}

func introductionInvocationPolicy(expected IntroductionExpected) invocationPolicy {
	program := filepath.Join(expected.InstallationRoot, "generations", expected.Generation, "ardents-node-linux-amd64")
	return invocationPolicy{program: program,
		arguments: strings.Join([]string{program, "introduction", "start-installed", filepath.Join(expected.InstallationRoot, "binding.json"), ""}, "\x00"),
		placement: "0::/system.slice/ardents-introduction.service\n"}
}

// Observe rechecks only original proc, executable, argv, credentials, protection,
// invocation and namespace objects. First failure permanently denies this pin.
// It grants no current State, manager, Guard, Startup or domain completion fact.
func (p IntroductionInvocation) Observe(ctx context.Context) error { return p.original.observe(ctx) }

// Close denies new uses synchronously, joins admitted physical observations and
// signal calls, then releases proc/namespaces and finally the original pidfd.
func (p IntroductionInvocation) Close() error { return p.original.close() }

func RetainIntroductionStop(ctx context.Context, expected IntroductionExpected, device, inode uint64) (IntroductionInvocation, error) {
	if device == 0 || inode == 0 {
		return IntroductionInvocation{}, errInput
	}
	p, err := RetainIntroduction(ctx, expected)
	if err != nil {
		return IntroductionInvocation{}, err
	}
	native, ok := p.original.pin.identity.Sys().(*syscall.Stat_t)
	if !ok || uint64(native.Dev) != device || native.Ino != inode {
		return IntroductionInvocation{}, errors.Join(errBinding, p.Close())
	}
	return p, nil
}

// InterruptIntroduction performs at most one SIGTERM through root's own original
// pidfd after fresh physical observations. The bool records a syscall attempt,
// including an uncertain failed attempt. Installation supplies stop permission;
// neither this signal nor process exit proves the original domains have joined.
func (p IntroductionInvocation) InterruptIntroduction(ctx context.Context) (bool, error) {
	return p.original.interrupt(ctx)
}
