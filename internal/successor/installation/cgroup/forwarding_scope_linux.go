package cgroup

import (
	"context"
	"os"
)

// ForwardingScope independently retains only the original fixed Forwarding
// kernel directory/events. Its caller serializes observations and joins them
// before Close. It grants no Source scope, process/manager, stop or Startup right.
type ForwardingScope struct{ original *processScope }

// CheckForwardingQuiescent observes only fresh absence or an empty fixed scope
// before a new effect. It cannot prove predecessor domain/native retirement.
func CheckForwardingQuiescent(ctx context.Context) error {
	return checkFixedQuiescent(ctx, "ardents-forwarding.service")
}

// RetainForwardingStarted is root's post-attempt physical acquisition. Partial
// custody must survive an observation failure until the original kernel join.
func RetainForwardingStarted(ctx context.Context, pid uint32) (*ForwardingScope, error) {
	if os.Geteuid() != 0 {
		return nil, errInput
	}
	return retainForwardingScope(ctx, pid, true)
}

// RetainForwardingSelf observes only the actual non-root calling process.
func RetainForwardingSelf(ctx context.Context) (*ForwardingScope, error) {
	if os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() {
		return nil, errInput
	}
	return retainForwardingScope(ctx, uint32(os.Getpid()), false)
}

// RetainForwardingStop separately reobserves archived original native identity.
// Copied identity values grant no accepting observation or interruption right.
func RetainForwardingStop(ctx context.Context, pid uint32, device, inode uint64) (*ForwardingScope, error) {
	if os.Geteuid() != 0 || device == 0 || inode == 0 {
		return nil, errInput
	}
	s, err := retainForwardingScope(ctx, pid, true)
	if err != nil {
		return s, err
	}
	native, ok := scopeNative(s.original.kernel.directory)
	if !ok || uint64(native.Dev) != device || native.Ino != inode {
		return s, errBinding
	}
	return s, nil
}

func retainForwardingScope(ctx context.Context, pid uint32, partial bool) (*ForwardingScope, error) {
	s, err := retainProcessScope(ctx, pid, partial, "ardents-forwarding.service")
	if s == nil {
		return nil, err
	}
	return &ForwardingScope{original: s}, err
}

// Observe matches the original directory, sole process and no child scopes.
func (s *ForwardingScope) Observe() error {
	if s == nil || s.original == nil {
		return errInput
	}
	return s.original.observe()
}

// Joined reads only the retained original events object, including after caller
// cancellation. Process exit, path absence and a new empty scope cannot join it.
func (s *ForwardingScope) Joined() (bool, error) {
	if s == nil || s.original == nil {
		return false, errInput
	}
	return s.original.joined()
}

// Close releases physical observations only. Root first establishes original
// native join; runtime joins its own users before its process leaves the scope.
func (s *ForwardingScope) Close() error {
	if s == nil || s.original == nil {
		return nil
	}
	return s.original.close()
}
