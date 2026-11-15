package cgroup

import (
	"context"
	"os"
)

// IntroductionScope independently retains only the original fixed Introduction
// kernel directory/events. Its caller serializes observations and joins them
// before Close. It grants no Source scope, process/manager, stop or Startup right.
type IntroductionScope struct{ original *processScope }

// RetainIntroductionStarted is root's post-attempt physical acquisition. Partial
// custody must survive an observation failure until the original kernel join.
func RetainIntroductionStarted(ctx context.Context, pid uint32) (*IntroductionScope, error) {
	if os.Geteuid() != 0 {
		return nil, errInput
	}
	return retainIntroductionScope(ctx, pid, true)
}

// RetainIntroductionSelf observes only the actual non-root calling process.
func RetainIntroductionSelf(ctx context.Context) (*IntroductionScope, error) {
	if os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() {
		return nil, errInput
	}
	return retainIntroductionScope(ctx, uint32(os.Getpid()), false)
}

func retainIntroductionScope(ctx context.Context, pid uint32, partial bool) (*IntroductionScope, error) {
	s, err := retainProcessScope(ctx, pid, partial, "ardents-introduction.service")
	if s == nil {
		return nil, err
	}
	return &IntroductionScope{original: s}, err
}

// Observe matches the original directory, sole process and no child scopes.
func (s *IntroductionScope) Observe() error {
	if s == nil || s.original == nil {
		return errInput
	}
	return s.original.observe()
}

// Joined reads only the retained original events object, including after caller
// cancellation. Process exit, path absence and a new empty scope cannot prove completion.
func (s *IntroductionScope) Joined() (bool, error) {
	if s == nil || s.original == nil {
		return false, errInput
	}
	return s.original.joined()
}

// Close releases physical observations only. Root first establishes original
// native join; runtime joins its own users before its process leaves the scope.
func (s *IntroductionScope) Close() error {
	if s == nil || s.original == nil {
		return nil
	}
	return s.original.close()
}

// RetainIntroductionStop separately reobserves archived original native identity.
// Copied identity values grant no accepting observation or interruption right.
func RetainIntroductionStop(ctx context.Context, pid uint32, device, inode uint64) (*IntroductionScope, error) {
	if os.Geteuid() != 0 || device == 0 || inode == 0 {
		return nil, errInput
	}
	s, err := retainIntroductionScope(ctx, pid, true)
	if err != nil {
		return s, err
	}
	native, ok := scopeNative(s.original.kernel.directory)
	if !ok || uint64(native.Dev) != device || native.Ino != inode {
		return s, errBinding
	}
	return s, nil
}
