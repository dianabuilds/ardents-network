package fixedfile

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// Mutation retains independent original descriptors, never an Installation
// lease or root. A nonnil result on opening failure still requires Close.
type Mutation struct {
	ctx        context.Context
	path       string
	parent     *os.Root
	file       *os.File
	parentInfo os.FileInfo
	original   os.FileInfo
	before     os.FileInfo
	preimage   []byte
	body       []byte
	mode       os.FileMode
	gid        uint32
	creation   bool
	attempted  bool
	closed     bool
	terminal   error
}

// Create exclusively births and synchronizes an empty root-owned leaf. Its
// Identity must be durably recorded by Installation before Commit is admitted.
func Create(ctx context.Context, filename string, parent os.FileInfo, body []byte, mode os.FileMode, gid uint32) (*Mutation, error) {
	owned, err := openParent(ctx, filename, parent, body, mode, gid)
	if err != nil {
		return owned, err
	}
	owned.creation = true
	owned.file, err = owned.parent.OpenFile(filepath.Base(filename), os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return owned, owned.fail(err)
	}
	birth, err := owned.file.Stat()
	pathBirth, pathErr := owned.parent.Lstat(filepath.Base(filename))
	if err != nil || pathErr != nil || !inodeMatches(birth, birth, 0600, 0) || birth.Size() != 0 || !sameFile(birth, pathBirth) {
		return owned, owned.fail(errors.Join(ErrBinding, err, pathErr))
	}
	owned.original, owned.before = birth, birth
	if err := errors.Join(owned.file.Sync(), syncParent(owned.parent)); err != nil {
		return owned, owned.fail(err)
	}
	if err := owned.ctx.Err(); err != nil {
		return owned, owned.fail(err)
	}
	return owned, nil
}

// Replace pins the recorded original inode and accepts only a complete old or
// candidate image, or a torn prefix of either. The caller must resynchronize
// its exact replacement record and recheck transaction admission before Commit.
func Replace(ctx context.Context, filename string, parent, original os.FileInfo, previous, candidate []byte, mode os.FileMode, gid uint32) (*Mutation, error) {
	if original == nil || len(previous) == 0 || len(previous) > 64<<20 {
		return nil, ErrInput
	}
	owned, err := openParent(ctx, filename, parent, candidate, mode, gid)
	if err != nil {
		return owned, err
	}
	owned.original = original
	owned.file, err = owned.parent.OpenFile(filepath.Base(filename), os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return owned, owned.fail(err)
	}
	before, err := owned.file.Stat()
	pathBefore, pathErr := owned.parent.Lstat(filepath.Base(filename))
	if err != nil || pathErr != nil || !inodeMatches(original, before, mode, gid) || !sameFile(before, pathBefore) {
		return owned, owned.fail(errors.Join(ErrBinding, err, pathErr))
	}
	current, err := io.ReadAll(io.LimitReader(owned.file, int64(max(len(previous), len(candidate)))+1))
	if err != nil || !ReplacementPrefixAllowed(current, previous, owned.body) {
		return owned, owned.fail(errors.Join(ErrBinding, err))
	}
	owned.before = before
	owned.preimage = bytes.Clone(current)
	if err := ctx.Err(); err != nil {
		return owned, owned.fail(err)
	}
	return owned, nil
}

func openParent(ctx context.Context, filename string, parent os.FileInfo, body []byte, mode os.FileMode, gid uint32) (*Mutation, error) {
	if ctx == nil || filename == "/" || !filepath.IsAbs(filename) || filepath.Clean(filename) != filename || parent == nil || len(body) == 0 || len(body) > 64<<20 || (mode != 0644 && mode != 0555 && mode != 0640) {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	owned := &Mutation{ctx: ctx, path: filename, parentInfo: parent, body: bytes.Clone(body), mode: mode, gid: gid}
	var err error
	owned.parent, err = os.OpenRoot(filepath.Dir(filename))
	if err != nil {
		return owned, owned.fail(err)
	}
	if err := owned.observeParent(); err != nil {
		return owned, owned.fail(err)
	}
	return owned, nil
}

// Identity supplies birth metadata, not an authorization or borrowed handle.
func (owned *Mutation) Identity() os.FileInfo {
	if owned == nil {
		return nil
	}
	return owned.original
}

// Commit performs one admitted mutation. It rechecks the original handles after
// the caller's journal I/O; another caller or a second Commit cannot renew it.
// A nonnil result on failure records visible same-inode bytes, not durability.
func (owned *Mutation) Commit() (result os.FileInfo, returnedErr error) {
	if owned == nil {
		return nil, ErrBinding
	}
	if owned.closed || owned.attempted || owned.file == nil || owned.before == nil || owned.terminal != nil {
		return nil, errors.Join(ErrBinding, owned.terminal)
	}
	defer func() {
		if returnedErr != nil {
			returnedErr = owned.fail(returnedErr)
		}
	}()
	if err := owned.ctx.Err(); err != nil {
		return nil, err
	}
	if err := owned.observeParent(); err != nil {
		return nil, err
	}
	current, err := owned.file.Stat()
	pathCurrent, pathErr := owned.parent.Lstat(filepath.Base(owned.path))
	if err != nil || pathErr != nil || !sameFile(owned.before, current) || !sameFile(owned.before, pathCurrent) {
		return nil, errors.Join(ErrBinding, err, pathErr)
	}
	// Native timestamps can coincide for distinct same-size writes. Re-read the
	// exact retained preimage after journal I/O before admitting mutation.
	if _, err := owned.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	preimage, err := io.ReadAll(io.LimitReader(owned.file, int64(len(owned.preimage))+1))
	if err != nil || !bytes.Equal(preimage, owned.preimage) {
		return nil, errors.Join(ErrBinding, err)
	}
	if err := owned.ctx.Err(); err != nil {
		return nil, err
	}
	owned.attempted = true
	// An already complete candidate still needs sync, but must not be truncated
	// again: an interruption there would put a torn file before later completed
	// files and destroy the transaction's ordered recovery prefix.
	if owned.creation || !bytes.Equal(owned.preimage, owned.body) {
		if !owned.creation {
			if err := owned.file.Truncate(0); err != nil {
				return nil, err
			}
			if _, err := owned.file.Seek(0, io.SeekStart); err != nil {
				return nil, err
			}
		}
		written, err := owned.file.Write(owned.body)
		if err != nil || written != len(owned.body) {
			return nil, errors.Join(io.ErrShortWrite, err)
		}
	}
	if owned.creation {
		if err := owned.file.Chown(0, int(owned.gid)); err != nil {
			return nil, err
		}
		if err := owned.file.Chmod(owned.mode); err != nil {
			return nil, err
		}
		if err := owned.file.Sync(); err != nil {
			return nil, err
		}
	}
	after, err := owned.file.Stat()
	pathAfter, pathErr := owned.parent.Lstat(filepath.Base(owned.path))
	if err != nil || pathErr != nil || !inodeMatches(owned.original, after, owned.mode, owned.gid) || !sameFile(after, pathAfter) || after.Size() != int64(len(owned.body)) {
		return nil, errors.Join(ErrBinding, err, pathErr)
	}
	if _, err := owned.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	actual, err := io.ReadAll(io.LimitReader(owned.file, int64(len(owned.body))+1))
	if err != nil || !bytes.Equal(actual, owned.body) {
		return nil, errors.Join(ErrBinding, err)
	}
	result = after
	if !owned.creation {
		if err := owned.file.Sync(); err != nil {
			return result, err
		}
	}
	if err := syncParent(owned.parent); err != nil {
		return result, err
	}
	final, err := owned.file.Stat()
	pathFinal, pathErr := owned.parent.Lstat(filepath.Base(owned.path))
	if err != nil || pathErr != nil || !sameFile(after, final) || !sameFile(after, pathFinal) {
		return result, errors.Join(ErrBinding, err, pathErr)
	}
	return result, errors.Join(owned.observeParent(), owned.ctx.Err())
}

func (owned *Mutation) observeParent() error {
	handle, err := owned.parent.Stat(".")
	pathInfo, pathErr := os.Lstat(filepath.Dir(owned.path))
	if err != nil || pathErr != nil || !sameDirectory(owned.parentInfo, handle) || !sameDirectory(owned.parentInfo, pathInfo) {
		return errors.Join(ErrBinding, err, pathErr)
	}
	return nil
}

func (owned *Mutation) fail(err error) error {
	owned.terminal = errors.Join(owned.terminal, err)
	return owned.terminal
}

// Close physically closes both original descriptors and retains every failure.
// No path is removed, and repeated Close returns the same retained outcome.
func (owned *Mutation) Close() error {
	if owned == nil {
		return nil
	}
	if !owned.closed {
		owned.closed = true
		if owned.file != nil {
			owned.terminal = errors.Join(owned.terminal, owned.file.Close())
			owned.file = nil
		}
		if owned.parent != nil {
			owned.terminal = errors.Join(owned.terminal, owned.parent.Close())
			owned.parent = nil
		}
		if owned.ctx == nil {
			owned.terminal = errors.Join(owned.terminal, ErrBinding)
		} else {
			owned.terminal = errors.Join(owned.terminal, owned.ctx.Err())
		}
	}
	return owned.terminal
}

func sameDirectory(original, current os.FileInfo) bool {
	if original == nil || current == nil || !current.IsDir() || !os.SameFile(original, current) || original.Mode() != current.Mode() || current.Mode().Perm()&0022 != 0 {
		return false
	}
	a, aOK := original.Sys().(*syscall.Stat_t)
	b, bOK := current.Sys().(*syscall.Stat_t)
	return aOK && bOK && a.Uid == 0 && b.Uid == 0 && a.Gid == b.Gid
}

func inodeMatches(original, current os.FileInfo, mode os.FileMode, gid uint32) bool {
	if original == nil || current == nil || !current.Mode().IsRegular() || !os.SameFile(original, current) || current.Mode() != mode {
		return false
	}
	native, ok := current.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == gid && native.Nlink == 1
}

func sameFile(original, current os.FileInfo) bool {
	if original == nil || current == nil || !os.SameFile(original, current) || original.Mode() != current.Mode() || original.Size() != current.Size() || !original.ModTime().Equal(current.ModTime()) {
		return false
	}
	a, aOK := original.Sys().(*syscall.Stat_t)
	b, bOK := current.Sys().(*syscall.Stat_t)
	return aOK && bOK && a.Uid == b.Uid && a.Gid == b.Gid && a.Nlink == 1 && b.Nlink == 1 && a.Ctim == b.Ctim
}

func syncParent(root *os.Root) (returnedErr error) {
	file, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	return file.Sync()
}
