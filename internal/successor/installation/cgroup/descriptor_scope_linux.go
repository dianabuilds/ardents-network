package cgroup

import (
	"context"
	"os"
)

// DescriptorScope independently retains only the original fixed Descriptor
// kernel directory/events. Its caller serializes observations and joins them
// before Close. It grants no Source scope, process/manager, stop or Startup right.
type DescriptorScope struct{ original *processScope }

// RetainDescriptorStarted is root's post-attempt physical acquisition. Partial
// custody must survive an observation failure until the original kernel join.
func RetainDescriptorStarted(ctx context.Context, pid uint32) (*DescriptorScope, error) {
	if os.Geteuid() != 0 {
		return nil, errInput
	}
	return retainDescriptorScope(ctx, pid, true)
}

// RetainDescriptorSelf observes only the actual non-root calling process.
func RetainDescriptorSelf(ctx context.Context) (*DescriptorScope, error) {
	if os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() {
		return nil, errInput
	}
	return retainDescriptorScope(ctx, uint32(os.Getpid()), false)
}

func retainDescriptorScope(ctx context.Context, pid uint32, partial bool) (*DescriptorScope, error) {
	s, err := retainProcessScope(ctx, pid, partial, "ardents-descriptor.service")
	if s == nil {
		return nil, err
	}
	return &DescriptorScope{original: s}, err
}

// Observe matches the original directory, sole process and no child scopes.
func (s *DescriptorScope) Observe() error {
	if s == nil || s.original == nil {
		return errInput
	}
	return s.original.observe()
}

// Joined reads only the retained original events object, including after caller
// cancellation. Process exit, path absence and a new empty scope cannot prove completion.
func (s *DescriptorScope) Joined() (bool, error) {
	if s == nil || s.original == nil {
		return false, errInput
	}
	return s.original.joined()
}

// Close releases physical observations only. Root first establishes original
// native join; runtime joins its own users before its process leaves the scope.
func (s *DescriptorScope) Close() error {
	if s == nil || s.original == nil {
		return nil
	}
	return s.original.close()
}
