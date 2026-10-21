package release

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// CheckHistoryPath matches a composition's canonical history declaration to
// this original verifier's directory and lease, then reobserves its floors.
// It lends no directory, lease or authorization. Installation retains its own native path custody;
// CheckAuthorizations separately checks the genuine exact-target results.
func (v *Verifier) CheckHistoryPath(ctx context.Context, path string) error {
	work, leave, err := v.enter(ctx)
	if err != nil {
		return err
	}
	defer leave()
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || path != v.store.path {
		return ErrTrustUnavailable
	}
	if err = v.store.checkHistoryIdentity(); err != nil {
		return err
	}
	if _, err = v.store.ReadFloors(); err != nil {
		return err
	}
	if err = v.store.checkHistoryIdentity(); err != nil {
		return err
	}
	err = errors.Join(ctx.Err(), work.Err())
	v.mu.Lock()
	if v.closed {
		err = errors.Join(err, ErrClosed)
	}
	v.mu.Unlock()
	return err
}

// The original descriptor was acquired with the verifier's native lease,
// before any authorization. Later consumer pins cannot recreate that identity.
func (s *floorStore) checkHistoryIdentity() (err error) {
	if s.failure != nil {
		return s.failure
	}
	defer func() {
		if err != nil {
			s.failure = err
		}
	}()
	l := &s.lease
	dir, e := l.directory.Stat()
	named, n := os.Lstat(s.path)
	if e != nil || n != nil || !os.SameFile(l.original, dir) || !os.SameFile(dir, named) ||
		!dir.IsDir() || dir.Mode() != l.original.Mode() || named.Mode() != dir.Mode() {
		return errors.Join(ErrTrustUnavailable, e, n)
	}
	before, ok := l.original.Sys().(*syscall.Stat_t)
	current, valid := dir.Sys().(*syscall.Stat_t)
	if !ok || !valid || before.Uid != current.Uid || before.Gid != current.Gid {
		return ErrTrustUnavailable
	}
	file, e := l.file.Stat()
	leaseName, n := os.Lstat(filepath.Join(s.path, floorStoreLockName))
	if e != nil || n != nil || !os.SameFile(file, leaseName) || !file.Mode().IsRegular() ||
		file.Mode() != leaseName.Mode() || file.Mode().Perm()&0077 != 0 {
		return errors.Join(ErrTrustUnavailable, e, n)
	}
	stat, ok := file.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) || stat.Nlink != 1 {
		return ErrTrustUnavailable
	}
	return nil
}
