package cgroup

import (
	"context"
	"errors"
	"io"
	"os"
	"syscall"
)

// CheckSourceRecoveryQuiescent is the independent read-only kernel observation
// for explicit Source producer abandonment. It grants no original completion,
// manager, State or participation authority. Only the fixed service principal
// range may observe it; State separately binds the exact named account, loaded
// stopped service, original manager and both exclusive semantic leases.
func CheckSourceRecoveryQuiescent(ctx context.Context) error {
	return observeFixedQuiescent(ctx, "ardents-source.service")
}

// Node roles share the ardents-node account. Each repair observes only its
// original unit scope so a sibling invocation does not block the repair.
// The caller separately checks the named account and the stopped unit.

// CheckForwardingRecoveryQuiescent observes the fixed Forwarding scope.
func CheckForwardingRecoveryQuiescent(ctx context.Context) error {
	return observeFixedQuiescent(ctx, "ardents-forwarding.service")
}

// CheckJoinRecoveryQuiescent observes the fixed JOIN scope.
func CheckJoinRecoveryQuiescent(ctx context.Context) error {
	return observeFixedQuiescent(ctx, "ardents-join.service")
}

// CheckDescriptorRecoveryQuiescent observes the fixed Descriptor scope.
func CheckDescriptorRecoveryQuiescent(ctx context.Context) error {
	return observeFixedQuiescent(ctx, "ardents-descriptor.service")
}

// CheckIntroductionRecoveryQuiescent observes the fixed Introduction scope.
func CheckIntroductionRecoveryQuiescent(ctx context.Context) error {
	return observeFixedQuiescent(ctx, "ardents-introduction.service")
}

func observeFixedQuiescent(ctx context.Context, unit string) (err error) {
	uid := os.Geteuid()
	if ctx == nil || uid < 100 || uid > 999 || os.Getegid() != uid {
		return errInput
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	const parentPath = "/sys/fs/cgroup/system.slice"
	scopePath := parentPath + "/" + unit
	if err = observeAncestors(parentPath); err != nil {
		return err
	}
	parent, err := os.OpenFile(parentPath, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, parent.Close(), ctx.Err()) }()
	before, err := parent.Stat()
	var fs syscall.Statfs_t
	fsErr := syscall.Fstatfs(int(parent.Fd()), &fs)
	if err != nil || fsErr != nil || !trustedDirectory(before) || fs.Type != cgroup2Magic {
		return errors.Join(errBinding, err, fsErr)
	}
	checkParent := func() error {
		current, e := parent.Stat()
		named, ne := os.Lstat(parentPath)
		if e != nil || ne != nil || !trustedDirectory(current) || !trustedDirectory(named) || !os.SameFile(before, current) || !os.SameFile(before, named) {
			return errors.Join(errBinding, e, ne)
		}
		return ctx.Err()
	}
	fd, err := syscall.Openat(int(parent.Fd()), unit, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if errors.Is(err, syscall.ENOENT) {
		if _, e := os.Lstat(scopePath); !errors.Is(e, os.ErrNotExist) {
			return errors.Join(errBinding, e)
		}
		return checkParent()
	}
	if err != nil {
		return err
	}
	directory := os.NewFile(uintptr(fd), "fresh fixed stopped scope")
	defer func() { err = errors.Join(err, directory.Close()) }()
	info, err := directory.Stat()
	if err != nil || !trustedDirectory(info) {
		return errors.Join(errBinding, err)
	}
	kernel, err := pinFixedScope(ctx, unit)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, kernel.close()) }()
	if !os.SameFile(info, kernel.directory) {
		return errBinding
	}
	check := func() error {
		removed, populated, e := kernel.read()
		if e != nil || removed || populated {
			return errors.Join(errBinding, e)
		}
		current, e := os.Lstat(scopePath)
		if e != nil || !trustedDirectory(current) || !os.SameFile(info, current) {
			return errors.Join(errBinding, e)
		}
		return checkParent()
	}
	if err = check(); err != nil {
		return err
	}
	entries, err := directory.ReadDir(257)
	if err != nil && err != io.EOF || len(entries) > 256 {
		return errors.Join(errBinding, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return errBinding
		}
	}
	return check()
}
