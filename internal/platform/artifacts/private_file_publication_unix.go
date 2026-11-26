//go:build linux || darwin

package artifacts

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/pathcheck"
)

// ErrOutputUncertain reports a failed publication after a possible effect.
var ErrOutputUncertain = errors.New("private artifact publication uncertain")

// privatePublicationDirectory and privatePublicationFile apply the same owner,
// mode and single-link checks on supported Unix hosts.
func privatePublicationDirectory(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && info.IsDir() && info.Mode().Perm() == 0700 && stat.Uid == uint32(os.Geteuid())
}

func privatePublicationFile(info os.FileInfo, size int64) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && info.Mode().IsRegular() && info.Mode().Perm() == 0600 && info.Size() == size && stat.Uid == uint32(os.Geteuid()) && stat.Nlink == 1
}

// CheckPrivateFileDestination checks access before a caller performs work
// that needs an owner-only output. Publication rechecks the original parent.
func CheckPrivateFileDestination(ctx context.Context, path string) error {
	_, err := privateOutputParent(ctx, path)
	return err
}

func privateOutputParent(ctx context.Context, path string) (os.FileInfo, error) {
	if ctx == nil || !pathcheck.IsCleanAbsolute(path) {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	parent := filepath.Dir(path)
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil || resolved != parent {
		return nil, ErrInput
	}
	info, err := os.Lstat(parent)
	if err != nil || !privatePublicationDirectory(info) {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return info, nil
}

// PublishPrivateFile writes one caller-owned file and syncs its original
// directory. An exact retry syncs the original file without replacing bytes.
func PublishPrivateFile(ctx context.Context, path string, raw []byte) (err error) {
	before, err := privateOutputParent(ctx, path)
	if err != nil {
		return err
	}
	parent := filepath.Dir(path)
	root, e := os.OpenRoot(parent)
	if e != nil {
		return ErrInput
	}
	changed := false
	defer func() {
		err = errors.Join(err, root.Close())
		if err != nil && changed {
			err = errors.Join(ErrOutputUncertain, err)
		}
	}()
	dir, e := root.Stat(".")
	if e != nil || !os.SameFile(before, dir) {
		return ErrInput
	}
	name := filepath.Base(path)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	f, e := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	fresh := e == nil
	if fresh {
		changed = true
	} else {
		if !errors.Is(e, os.ErrExist) {
			return ErrInput
		}
		info, statErr := root.Lstat(name)
		if statErr != nil || !privatePublicationFile(info, int64(len(raw))) {
			return ErrInput
		}
		f, e = root.OpenFile(name, os.O_RDWR, 0)
		if e != nil {
			return ErrInput
		}
		opened, statErr := f.Stat()
		if statErr != nil || !os.SameFile(info, opened) || !privatePublicationFile(opened, opened.Size()) {
			return errors.Join(ErrInput, f.Close())
		}
		saved, readErr := io.ReadAll(io.LimitReader(f, int64(len(raw))+1))
		if readErr != nil || !bytes.Equal(saved, raw) {
			return errors.Join(ErrInput, f.Close())
		}
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	if fresh {
		n, e := f.Write(raw)
		if e != nil {
			return e
		}
		if n != len(raw) {
			return io.ErrShortWrite
		}
	}
	// Exact retries complete the durability barrier, without replacing bytes.
	if e = f.Sync(); e != nil {
		changed = true
		return e
	}
	opened, e := f.Stat()
	after, statErr := root.Lstat(name)
	if e != nil || statErr != nil || !os.SameFile(opened, after) || !privatePublicationFile(after, int64(len(raw))) {
		return ErrInput
	}
	d, e := root.Open(".")
	if e != nil {
		return e
	}
	e = errors.Join(d.Sync(), d.Close())
	if e != nil {
		changed = true
		return e
	}
	final, e := os.Lstat(parent)
	if e != nil || !os.SameFile(before, final) || !privatePublicationDirectory(final) {
		return ErrInput
	}
	return nil
}
