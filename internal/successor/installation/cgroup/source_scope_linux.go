package cgroup

import (
	"context"
	"os"
)

// SourceScope independently retains the fixed original Source kernel scope.
// Its caller serializes observations, joins those users before Close, and owns
// process/manager admission and stopping. Joined reads only the original events
// descriptor; neither path absence nor another empty cgroup supplies completion.
type SourceScope struct{ original *processScope }

// RetainSourceStarted is root's physical observation after its own fixed Source
// start attempt. Partial custody survives an observation failure and must remain
// retained until original join. PID zero cannot supply successful admission.
func RetainSourceStarted(ctx context.Context, pid uint32) (*SourceScope, error) {
	if os.Geteuid() != 0 {
		return nil, errInput
	}
	return retainSourceScope(ctx, pid, true)
}

// RetainSourceStop independently observes an archived original scope before a
// new explicit stop. Before any interruption it is read-only observation;
// after an attempted effect Installation retains it through actual Joined.
func RetainSourceStop(ctx context.Context, pid uint32, device, inode uint64) (*SourceScope, error) {
	if os.Geteuid() != 0 || device == 0 || inode == 0 {
		return nil, errInput
	}
	scope, err := retainSourceScope(ctx, pid, true)
	if err != nil {
		return scope, err
	}
	native, ok := scopeNative(scope.original.kernel.directory)
	if !ok || uint64(native.Dev) != device || native.Ino != inode {
		return scope, errBinding
	}
	return scope, nil
}

// RetainSourceSelf observes only this actual non-root calling process. It owns
// no root transition lease, stop right or foreign process selection.
func RetainSourceSelf(ctx context.Context) (*SourceScope, error) {
	if os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() {
		return nil, errInput
	}
	return retainSourceScope(ctx, uint32(os.Getpid()), false)
}

func retainSourceScope(ctx context.Context, pid uint32, partial bool) (*SourceScope, error) {
	original, err := retainProcessScope(ctx, pid, partial, "ardents-source.service")
	if original == nil {
		return nil, err
	}
	return &SourceScope{original: original}, err
}

// Observe checks the same original directory, sole process and absence of child
// scopes. It never follows another PID or adopts a replacement path.
func (s *SourceScope) Observe() error {
	if s == nil || s.original == nil {
		return errInput
	}
	return s.original.observe()
}

// Joined remains available for physical cleanup after original caller denial.
// It grants no accepting observation and never replaces original custody.
func (s *SourceScope) Joined() (bool, error) {
	if s == nil || s.original == nil {
		return false, errInput
	}
	return s.original.joined()
}

// Close releases observations only; root must establish Joined first. Runtime
// closes its own read-only observer after its users join, before its process exits.
func (s *SourceScope) Close() error {
	if s == nil || s.original == nil {
		return nil
	}
	return s.original.close()
}
