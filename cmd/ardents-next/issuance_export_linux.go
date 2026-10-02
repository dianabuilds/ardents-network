//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

func ownedIssuanceOutput(info os.FileInfo, dir bool) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	if dir {
		return info.IsDir() && info.Mode().Perm() == 0700
	}
	return info.Mode().IsRegular() && info.Mode().Perm() == 0600 && stat.Nlink == 1
}

// Check before domain effects, then again immediately before opening output.
// All issuance outputs share this rule, including inventories and profiles.
func checkIssuanceOutput(ctx context.Context, path string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return issuance.ErrInvalid
	}
	for directory := filepath.Dir(path); ; directory = filepath.Dir(directory) {
		for _, marker := range []string{"identity.pin", "identity.key", "identity.lock", "identity.pending", "profile.pin", "profile.bytes", "profile.lock", "profile.pending", "issuer.pin", "issuer.keys", "issuer.lock", "issuer.pending", "admission.pin", "admission.lock", "admission.journal", "admission.floor", "admission.pending", "results.pin", "results.lock", "results.journal", "results.floor", "results.pending", "budget.pin", "budget.lock", "budget.json", "budget.pending"} {
			if _, err := os.Lstat(filepath.Join(directory, marker)); !errors.Is(err, os.ErrNotExist) {
				return issuance.ErrUnavailable
			}
		}
		if filepath.Dir(directory) == directory {
			break
		}
	}
	parent := filepath.Dir(path)
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil || resolved != parent {
		return issuance.ErrUnavailable
	}
	info, err := os.Lstat(parent)
	if err != nil || !ownedIssuanceOutput(info, true) {
		return issuance.ErrUnavailable
	}
	return ctx.Err()
}

func exportIssuanceOutput(ctx context.Context, path string, raw []byte) (err error) {
	if err := checkIssuanceOutput(ctx, path); err != nil {
		return err
	}
	parent := filepath.Dir(path)
	resolved, e := filepath.EvalSymlinks(parent)
	if e != nil || resolved != parent {
		return issuance.ErrUnavailable
	}
	before, e := os.Lstat(parent)
	if e != nil || !ownedIssuanceOutput(before, true) {
		return issuance.ErrUnavailable
	}
	root, e := os.OpenRoot(parent)
	if e != nil {
		return issuance.ErrUnavailable
	}
	changed := false
	defer func() {
		err = errors.Join(err, root.Close())
		if err != nil && changed {
			err = errors.Join(issuance.ErrUncertain, err)
		}
	}()
	dir, e := root.Stat(".")
	if e != nil || !os.SameFile(before, dir) {
		return issuance.ErrUnavailable
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
			return issuance.ErrUnavailable
		}
		info, statErr := root.Lstat(name)
		if statErr != nil || !ownedIssuanceOutput(info, false) || info.Size() != int64(len(raw)) {
			return issuance.ErrUnavailable
		}
		f, e = root.OpenFile(name, os.O_RDWR, 0)
		if e != nil {
			return issuance.ErrUnavailable
		}
		opened, statErr := f.Stat()
		if statErr != nil || !os.SameFile(info, opened) || !ownedIssuanceOutput(opened, false) {
			return errors.Join(issuance.ErrUnavailable, f.Close())
		}
		saved, readErr := io.ReadAll(io.LimitReader(f, int64(len(raw))+1))
		if readErr != nil || !bytes.Equal(saved, raw) {
			return errors.Join(issuance.ErrUnavailable, f.Close())
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
	if e != nil || statErr != nil || !os.SameFile(opened, after) || !ownedIssuanceOutput(after, false) || after.Size() != int64(len(raw)) {
		return issuance.ErrUnavailable
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
	if e != nil || !os.SameFile(before, final) || !ownedIssuanceOutput(final, true) {
		return issuance.ErrUnavailable
	}
	return nil
}
