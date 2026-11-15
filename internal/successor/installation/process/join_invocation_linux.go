package process

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// JoinExpected contains independently checked installation/manager facts,
// never an accepting startup proof. No input chooses another Node duty or unit.
type JoinExpected struct {
	PID, UID, GID                uint32
	Invocation                   [16]byte
	InstallationRoot, Generation string
	ProgramIdentity              os.FileInfo
}

// JoinInvocation retains its own original kernel observations. Copies
// share admission, the first failure and physically joined Close. It exposes
// neither the underlying Invocation nor Source's original signal right.
type JoinInvocation struct{ original *nodeInvocation }

// RetainJoin independently observes root's exact original Join
// process. Installation separately admits start/stop and retains cgroup join.
func RetainJoin(ctx context.Context, expected JoinExpected) (JoinInvocation, error) {
	if os.Geteuid() != 0 {
		return JoinInvocation{}, errInput
	}
	return retainJoin(ctx, expected)
}

func RetainJoinStop(ctx context.Context, expected JoinExpected, device, inode uint64) (JoinInvocation, error) {
	if device == 0 || inode == 0 {
		return JoinInvocation{}, errInput
	}
	p, err := RetainJoin(ctx, expected)
	if err != nil {
		return JoinInvocation{}, err
	}
	if err := p.original.matchArchived(device, inode); err != nil {
		return JoinInvocation{}, err
	}
	return p, nil
}

// RetainJoinSelf observes only this actual non-root calling process.
// Root's observation cannot be passed in as a substitute for this acquisition.
func RetainJoinSelf(ctx context.Context, expected JoinExpected) (JoinInvocation, error) {
	if os.Geteuid() == 0 || expected.PID != uint32(os.Getpid()) ||
		expected.UID != uint32(os.Geteuid()) || expected.GID != uint32(os.Getegid()) {
		return JoinInvocation{}, errInput
	}
	return retainJoin(ctx, expected)
}

func retainJoin(ctx context.Context, expected JoinExpected) (JoinInvocation, error) {
	o, err := retainNode(ctx, Expected(expected), joinInvocationPolicy(expected), "original Join pidfd")
	if err != nil {
		return JoinInvocation{}, err
	}
	return JoinInvocation{original: o}, nil
}

func joinInvocationPolicy(expected JoinExpected) invocationPolicy {
	program := filepath.Join(expected.InstallationRoot, "generations", expected.Generation, "ardents-node-linux-amd64")
	return invocationPolicy{program: program,
		arguments: strings.Join([]string{program, "join", "start-installed", filepath.Join(expected.InstallationRoot, "binding.json"), ""}, "\x00"),
		placement: "0::/system.slice/ardents-join.service\n"}
}

// Observe rechecks only original proc, executable, argv, credentials, protection,
// invocation and namespace objects. First failure permanently denies this pin.
// It grants no current State, manager, Guard, Startup or domain completion fact.
func (p JoinInvocation) Observe(ctx context.Context) error { return p.original.observe(ctx) }

// InterruptJoin performs at most one SIGTERM through root's own original
// pidfd after fresh physical observations. The bool records a syscall attempt,
// including an uncertain failed attempt. Installation supplies stop permission;
// neither this signal nor process exit proves the original domains have joined.
func (p JoinInvocation) InterruptJoin(ctx context.Context) (bool, error) {
	return p.original.interrupt(ctx)
}

// Close denies new uses synchronously, joins admitted physical observations and
// signal calls, then releases proc/namespaces and finally the original pidfd.
func (p JoinInvocation) Close() error { return p.original.close() }
