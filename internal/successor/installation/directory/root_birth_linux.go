package directory

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// RootBirth retains one exclusively born root0700 directory and independent
// original parent custody. Installation serializes its operations and records
// Identity before Deliver or SealRoot. No method adopts an existing leaf or
// grants path, Release, account, journal, generation or startup permission.
type RootBirth struct {
	ctx        context.Context
	path       string
	parent     *os.Root
	parentInfo os.FileInfo
	file       *os.File
	before     os.FileInfo
	attempted  bool
	delivered  bool
	sealing    bool
	closed     bool
	failure    error
}

// CreateRoot returns partial custody on every post-opening failure. A nonnil
// result always requires Close. The caller admits the exact parent and path;
// this mechanism checks their actual native identity and empty exclusive birth.
func CreateRoot(ctx context.Context, filename string, parent os.FileInfo) (result *RootBirth, returned error) {
	if ctx == nil || os.Geteuid() != 0 || !canonicalPath(filename) || filename == "/" || !rootBirthParent(parent) {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(filepath.Dir(filename))
	if err != nil {
		return nil, err
	}
	b := &RootBirth{ctx: ctx, path: filename, parent: root, parentInfo: parent}
	defer func() { b.record(returned) }()
	if err := b.observeParent(); err != nil {
		return b, err
	}
	var storage syscall.Statfs_t
	if err := syscall.Statfs(filepath.Dir(filename), &storage); err != nil || storage.Type != 0xef53 || storage.Flags&1 != 0 {
		return b, errors.Join(ErrNativeUnavailable, err)
	}
	name := filepath.Base(filename)
	if err := root.Mkdir(name, 0700); err != nil {
		return b, err
	}
	b.file, err = root.OpenFile(name, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return b, err
	}
	b.before, err = b.file.Stat()
	if err != nil || !rootBirthAccess(b.before, 0700, 0, 0) {
		return b, errors.Join(ErrBinding, err)
	}
	if err := errors.Join(b.file.Sync(), b.syncParent(), b.Observe()); err != nil {
		return b, err
	}
	return b, nil
}

func rootBirthParent(info os.FileInfo) bool {
	if info == nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == 0
}

func rootBirthAccess(info os.FileInfo, mode os.FileMode, uid, gid uint32) bool {
	if info == nil || !info.IsDir() || info.Mode() != os.ModeDir|mode {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == uid && native.Gid == gid
}

func (b *RootBirth) record(err error) {
	if err != nil && b.failure == nil {
		b.failure = err
	}
}

func (b *RootBirth) observeParent() error {
	handle, err := b.parent.Stat(".")
	named, nameErr := os.Lstat(filepath.Dir(b.path))
	if err != nil || nameErr != nil || !rootBirthParent(handle) || !rootBirthParent(named) || !os.SameFile(b.parentInfo, handle) || !os.SameFile(b.parentInfo, named) || b.parentInfo.Mode() != handle.Mode() || b.parentInfo.Mode() != named.Mode() {
		return errors.Join(ErrBinding, err, nameErr)
	}
	return b.ctx.Err()
}

func (b *RootBirth) syncParent() (returned error) {
	f, err := b.parent.Open(".")
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, f.Close()) }()
	return f.Sync()
}

// Observe rechecks the same inode/account/mode and original parent. Directory
// contents and ctime belong to the caller's phase inventory, not an immutable
// snapshot: admitted child births legitimately change both.
func (b *RootBirth) Observe() (returned error) {
	if b == nil || b.ctx == nil || b.closed || b.file == nil || b.before == nil {
		return ErrBinding
	}
	if b.failure != nil {
		return b.failure
	}
	defer func() { b.record(returned) }()
	if err := b.ctx.Err(); err != nil {
		return err
	}
	if err := b.observeParent(); err != nil {
		return err
	}
	handle, err := b.file.Stat()
	named, nameErr := b.parent.Lstat(filepath.Base(b.path))
	st, ok := b.before.Sys().(*syscall.Stat_t)
	if !ok || err != nil || nameErr != nil || !os.SameFile(b.before, handle) || !os.SameFile(b.before, named) || !rootBirthAccess(handle, b.before.Mode().Perm(), st.Uid, st.Gid) || !rootBirthAccess(named, b.before.Mode().Perm(), st.Uid, st.Gid) {
		return errors.Join(ErrBinding, err, nameErr)
	}
	return b.ctx.Err()
}

// Identity copies checked physical metadata, not birth or effect authority.
func (b *RootBirth) Identity() (os.FileInfo, error) {
	if err := b.Observe(); err != nil {
		return nil, err
	}
	return b.before, nil
}

// Deliver attempts one same-inode account/mode promotion. Installation records
// its original empty birth before calling this method; uncertainty is retained
// and cannot be retried. It selects no account, role or resource policy.
func (b *RootBirth) Deliver(mode os.FileMode, uid, gid uint32) (result os.FileInfo, returned error) {
	if b == nil || b.attempted || mode != 0700 && mode != 0750 && mode != 0755 {
		return nil, ErrInput
	}
	if err := b.Observe(); err != nil {
		return nil, err
	}
	defer func() { b.record(returned) }()
	entries, err := b.file.ReadDir(1)
	if len(entries) != 0 || err != io.EOF {
		return nil, errors.Join(ErrBinding, err)
	}
	b.attempted = true
	if err := b.file.Chown(int(uid), int(gid)); err != nil {
		return nil, err
	}
	if err := errors.Join(b.file.Chmod(mode), b.file.Sync()); err != nil {
		return nil, err
	}
	after, err := b.file.Stat()
	if err != nil || !os.SameFile(b.before, after) || !rootBirthAccess(after, mode, uid, gid) {
		return nil, errors.Join(ErrBinding, err)
	}
	b.before = after
	if err := b.Observe(); err != nil {
		return nil, err
	}
	b.delivered = true
	return after, nil
}

// SealRoot permits one read-only seal of this operation's delivered root0755
// generation directory. Installation owns its complete file inventory and
// seal-intent record; this physical mechanism grants neither.
func (b *RootBirth) SealRoot() (result os.FileInfo, returned error) {
	if b == nil || !b.delivered || b.sealing || !rootBirthAccess(b.before, 0755, 0, 0) {
		return nil, ErrInput
	}
	if err := b.Observe(); err != nil {
		return nil, err
	}
	defer func() { b.record(returned) }()
	b.sealing = true
	if err := errors.Join(b.file.Chmod(0555), b.file.Sync()); err != nil {
		return nil, err
	}
	after, err := b.file.Stat()
	if err != nil || !os.SameFile(b.before, after) || !rootBirthAccess(after, 0555, 0, 0) {
		return nil, errors.Join(ErrBinding, err)
	}
	b.before = after
	return after, b.Observe()
}

// Close retains all physical failures and partial prefixes; it removes nothing.
func (b *RootBirth) Close() error {
	if b == nil {
		return nil
	}
	if !b.closed {
		b.closed = true
		if b.file != nil {
			b.failure = errors.Join(b.failure, b.file.Close())
		}
		if b.parent != nil {
			b.failure = errors.Join(b.failure, b.parent.Close())
		}
		if b.ctx == nil {
			b.failure = errors.Join(b.failure, ErrInput)
		} else {
			b.failure = errors.Join(b.failure, b.ctx.Err())
		}
	}
	return b.failure
}
