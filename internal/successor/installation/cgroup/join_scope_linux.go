package cgroup

import (
	"context"
	"os"
)

// JoinScope independently retains only the original fixed Join
// kernel directory/events. Its caller serializes observations and joins them
// before Close. It grants no Source scope, process/manager, stop or Startup right.
type JoinScope struct{ original *processScope }

// RetainJoinStarted is root's post-attempt physical acquisition. Partial
// custody must survive an observation failure until the original kernel join.
func RetainJoinStarted(ctx context.Context, pid uint32) (*JoinScope, error) {
	if os.Geteuid() != 0 {
		return nil, errInput
	}
	return retainJoinScope(ctx, pid, true)
}

// RetainJoinSelf observes only the actual non-root calling process.
func RetainJoinSelf(ctx context.Context) (*JoinScope, error) {
	if os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() {
		return nil, errInput
	}
	return retainJoinScope(ctx, uint32(os.Getpid()), false)
}

// RetainJoinStop separately reobserves archived original native identity.
// Copied identity values grant no accepting observation or interruption right.
func RetainJoinStop(ctx context.Context, pid uint32, device, inode uint64) (*JoinScope, error) {
	if os.Geteuid() != 0 || device == 0 || inode == 0 {
		return nil, errInput
	}
	s, err := retainJoinScope(ctx, pid, true)
	if err != nil {
		return s, err
	}
	native, ok := scopeNative(s.original.kernel.directory)
	if !ok || uint64(native.Dev) != device || native.Ino != inode {
		return s, errBinding
	}
	return s, nil
}

func retainJoinScope(ctx context.Context, pid uint32, partial bool) (*JoinScope, error) {
	s, err := retainProcessScope(ctx, pid, partial, "ardents-join.service")
	if s == nil {
		return nil, err
	}
	return &JoinScope{original: s}, err
}

// Observe matches the original directory, sole process and no child scopes.
func (s *JoinScope) Observe() error {
	if s == nil || s.original == nil {
		return errInput
	}
	return s.original.observe()
}

// Joined reads only the retained original events object, including after caller
// cancellation. Process exit, path absence and a new empty scope cannot join it.
func (s *JoinScope) Joined() (bool, error) {
	if s == nil || s.original == nil {
		return false, errInput
	}
	return s.original.joined()
}

// Close releases physical observations only. Root first establishes original
// native join; runtime joins its own users before its process leaves the scope.
func (s *JoinScope) Close() error {
	if s == nil || s.original == nil {
		return nil
	}
	return s.original.close()
}
