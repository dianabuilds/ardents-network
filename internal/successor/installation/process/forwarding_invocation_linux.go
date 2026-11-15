package process

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// ForwardingExpected contains independently checked installation/manager facts,
// never an accepting startup proof. No input chooses another Node duty or unit.
type ForwardingExpected struct {
	PID, UID, GID                uint32
	Invocation                   [16]byte
	InstallationRoot, Generation string
	ProgramIdentity              os.FileInfo
}

// ForwardingInvocation retains its own original kernel observations. Copies
// share admission, the first failure and physically joined Close. It exposes
// neither the underlying Invocation nor Source's original signal right.
type ForwardingInvocation struct{ original *nodeInvocation }

// RetainForwarding independently observes root's exact original Forwarding
// process. Installation separately admits start/stop and retains cgroup join.
func RetainForwarding(ctx context.Context, expected ForwardingExpected) (ForwardingInvocation, error) {
	if os.Geteuid() != 0 {
		return ForwardingInvocation{}, errInput
	}
	return retainForwarding(ctx, expected)
}

// RetainForwardingStop additionally matches the archived original proc inode.
// Copied inode facts grant no signal right: the fixed executable, argv, actual
// principal, namespaces, InvocationID and original pidfd are observed afresh.
func RetainForwardingStop(ctx context.Context, expected ForwardingExpected, device, inode uint64) (ForwardingInvocation, error) {
	if device == 0 || inode == 0 {
		return ForwardingInvocation{}, errInput
	}
	p, err := RetainForwarding(ctx, expected)
	if err != nil {
		return ForwardingInvocation{}, err
	}
	if err := p.original.matchArchived(device, inode); err != nil {
		return ForwardingInvocation{}, err
	}
	return p, nil
}

// RetainForwardingSelf observes only this actual non-root calling process.
// Root's observation cannot be passed in as a substitute for this acquisition.
func RetainForwardingSelf(ctx context.Context, expected ForwardingExpected) (ForwardingInvocation, error) {
	if os.Geteuid() == 0 || expected.PID != uint32(os.Getpid()) ||
		expected.UID != uint32(os.Geteuid()) || expected.GID != uint32(os.Getegid()) {
		return ForwardingInvocation{}, errInput
	}
	return retainForwarding(ctx, expected)
}

func retainForwarding(ctx context.Context, expected ForwardingExpected) (ForwardingInvocation, error) {
	o, err := retainNode(ctx, Expected(expected), forwardingInvocationPolicy(expected), "original Forwarding pidfd")
	if err != nil {
		return ForwardingInvocation{}, err
	}
	return ForwardingInvocation{original: o}, nil
}

func forwardingInvocationPolicy(expected ForwardingExpected) invocationPolicy {
	program := filepath.Join(expected.InstallationRoot, "generations", expected.Generation, "ardents-node-linux-amd64")
	return invocationPolicy{program: program,
		arguments: strings.Join([]string{program, "forwarding", "start-installed", filepath.Join(expected.InstallationRoot, "binding.json"), ""}, "\x00"),
		placement: "0::/system.slice/ardents-forwarding.service\n"}
}

// Observe rechecks only original proc, executable, argv, credentials, protection,
// invocation and namespace objects. First failure permanently denies this pin.
// It grants no current State, manager, Guard, Startup or domain completion fact.
func (p ForwardingInvocation) Observe(ctx context.Context) error { return p.original.observe(ctx) }

// InterruptForwarding performs at most one SIGTERM through root's own original
// pidfd after fresh physical observations. The bool records a syscall attempt,
// including an uncertain failed attempt. Installation supplies stop permission;
// neither this signal nor process exit proves the original domains have joined.
func (p ForwardingInvocation) InterruptForwarding(ctx context.Context) (bool, error) {
	return p.original.interrupt(ctx)
}

// Close denies new uses synchronously, joins admitted physical observations and
// signal calls, then releases proc/namespaces and finally the original pidfd.
func (p ForwardingInvocation) Close() error { return p.original.close() }
