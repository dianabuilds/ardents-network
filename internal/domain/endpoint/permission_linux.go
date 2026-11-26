//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/pathcheck"
)

// Publish once, or resynchronize an exact retry. A partial or foreign request
// is retained as a refusal; it is never replaced with a new holder's request.
func installedPermissionRequest(path string, raw []byte, check func() error) (err error) {
	if check == nil || len(raw) == 0 || len(raw) > 4096 || !pathcheck.IsCleanAbsolute(path) {
		return errors.New("invalid installed permission handover")
	}
	if err := check(); err != nil {
		return err
	}
	parent := filepath.Dir(path)
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil || resolved != parent {
		return errors.Join(errors.New("permission parent unavailable"), err)
	}
	before, err := os.Lstat(parent)
	if err != nil || !installedPermissionOwned(before, true) {
		return errors.Join(errors.New("permission parent unavailable"), err)
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	current := func() error {
		if err := check(); err != nil {
			return err
		}
		opened, a := root.Stat(".")
		linked, b := os.Lstat(parent)
		if a != nil || b != nil || !os.SameFile(before, opened) || !os.SameFile(before, linked) || !installedPermissionOwned(linked, true) {
			return errors.Join(errors.New("permission parent changed"), a, b)
		}
		return nil
	}
	if err := current(); err != nil {
		return err
	}
	name := filepath.Base(path)
	file, err := root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	fresh := err == nil
	if !fresh {
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		before, err := root.Lstat(name)
		if err != nil || !installedPermissionOwned(before, false) || before.Size() != int64(len(raw)) {
			return errors.Join(errors.New("permission request changed"), err)
		}
		file, err = root.OpenFile(name, os.O_RDWR|syscall.O_NOFOLLOW, 0)
		if err != nil {
			return err
		}
		opened, err := file.Stat()
		if err != nil || !os.SameFile(before, opened) || !installedPermissionOwned(opened, false) {
			return errors.Join(errors.New("permission request changed"), err, file.Close())
		}
		retained, err := io.ReadAll(io.LimitReader(file, int64(len(raw))+1))
		if err != nil || !bytes.Equal(retained, raw) {
			return errors.Join(errors.New("permission request differs"), err, file.Close())
		}
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	if err := current(); err != nil {
		return err
	}
	if fresh {
		count, err := file.Write(raw)
		if err != nil {
			return err
		}
		if count != len(raw) {
			return io.ErrShortWrite
		}
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	retained, err := io.ReadAll(io.LimitReader(file, int64(len(raw))+1))
	if err != nil || !bytes.Equal(retained, raw) {
		return errors.Join(errors.New("permission request changed during durability"), err)
	}
	opened, a := file.Stat()
	linked, b := root.Lstat(name)
	if a != nil || b != nil || !os.SameFile(opened, linked) || !installedPermissionOwned(linked, false) || linked.Size() != int64(len(raw)) {
		return errors.Join(errors.New("permission request changed"), a, b)
	}
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	if err := errors.Join(directory.Sync(), directory.Close()); err != nil {
		return err
	}
	return current()
}

// installedPermissionRetire retires a prior invocation's permission handover so a
// fresh platform-led invocation can publish its own fresh holder request. The
// volatile holder is erased at shutdown, so a restart requires a fresh holder and a
// fresh external authority approval, never a revival of the preceding allocation.
// Retirement removes only endpoint-owned regular handover files under the original
// private parent, response before request so a stale grant can never pair with a
// fresh request. A foreign, partial or substituted handover is retained as a
// refusal and never removed. Retirement grants no authority: the external permission
// authority must still sign the fresh request, and the durable presentation journal
// retains every prior spend. The within-invocation publish rule
// (installedPermissionRequest) is unchanged; this is a distinct bootstrap-phase
// lifecycle step, and at bootstrap any existing handover necessarily belongs to a
// prior invocation because this process has not published yet.
func installedPermissionRetire(requestPath, responsePath string, check func() error) (err error) {
	if check == nil || !pathcheck.IsCleanAbsolute(requestPath) ||
		!pathcheck.IsCleanAbsolute(responsePath) {
		return errors.New("invalid installed permission handover")
	}
	parent := filepath.Dir(requestPath)
	if filepath.Dir(responsePath) != parent {
		return errors.New("invalid installed permission handover")
	}
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil || resolved != parent {
		return errors.Join(errors.New("permission parent unavailable"), err)
	}
	before, err := os.Lstat(parent)
	if err != nil || !installedPermissionOwned(before, true) {
		return errors.Join(errors.New("permission parent unavailable"), err)
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	current := func() error {
		if err := check(); err != nil {
			return err
		}
		opened, a := root.Stat(".")
		linked, b := os.Lstat(parent)
		if a != nil || b != nil || !os.SameFile(before, opened) || !os.SameFile(before, linked) || !installedPermissionOwned(linked, true) {
			return errors.Join(errors.New("permission parent changed"), a, b)
		}
		return nil
	}
	if err := current(); err != nil {
		return err
	}
	for _, name := range []string{filepath.Base(responsePath), filepath.Base(requestPath)} {
		linked, err := root.Lstat(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || !installedPermissionOwned(linked, false) {
			return errors.Join(errors.New("permission handover changed"), err)
		}
		if err := root.Remove(name); err != nil {
			return errors.Join(errors.New("permission handover retirement failed"), err)
		}
		if err := current(); err != nil {
			return err
		}
	}
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	if err := errors.Join(directory.Sync(), directory.Close()); err != nil {
		return err
	}
	return current()
}

func installedPermissionRead(path string) (raw []byte, err error) {
	if !pathcheck.IsCleanAbsolute(path) {
		return nil, errors.New("invalid permission response path")
	}
	parent := filepath.Dir(path)
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil || resolved != parent {
		return nil, errors.Join(errors.New("permission parent unavailable"), err)
	}
	before, err := os.Lstat(parent)
	if err != nil || !installedPermissionOwned(before, true) {
		return nil, errors.Join(errors.New("permission parent unavailable"), err)
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, root.Close())
		if err != nil {
			clear(raw)
			raw = nil
		}
	}()
	name := filepath.Base(path)
	linked, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !installedPermissionOwned(linked, false) || linked.Size() != 228 {
		return nil, errors.New("permission response unavailable")
	}
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(linked, opened) || !installedPermissionOwned(opened, false) {
		return nil, errors.Join(errors.New("permission response changed"), err)
	}
	raw, err = io.ReadAll(io.LimitReader(file, 229))
	if err != nil || len(raw) != 228 {
		return nil, errors.Join(errors.New("permission response incomplete"), err)
	}
	after, a := root.Lstat(name)
	parentAfter, b := os.Lstat(parent)
	rootAfter, c := root.Stat(".")
	if a != nil || b != nil || c != nil || !os.SameFile(opened, after) || !installedPermissionOwned(after, false) || !os.SameFile(before, parentAfter) || !os.SameFile(before, rootAfter) || !installedPermissionOwned(parentAfter, true) {
		return nil, errors.Join(errors.New("permission handover changed"), a, b, c)
	}
	return raw, nil
}

func installedPermissionOwned(info os.FileInfo, directory bool) bool {
	if info == nil {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return false
	}
	if directory {
		return info.Mode() == os.ModeDir|0700
	}
	return info.Mode() == 0600 && stat.Nlink == 1
}

func installedPermissionResponse(ctx context.Context, path string, digest [32]byte, deadline time.Time, holder *stock.Owner, check func() error) error {
	bounded, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		if err := check(); err != nil {
			return err
		}
		raw, err := installedPermissionRead(path)
		if err == nil {
			if err := check(); err != nil {
				clear(raw)
				return err
			}
			err = holder.Import(digest, raw)
			clear(raw)
			return errors.Join(err, check())
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		select {
		case <-bounded.Done():
			return bounded.Err()
		case <-tick.C:
		}
	}
}
