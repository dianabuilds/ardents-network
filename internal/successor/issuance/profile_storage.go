package issuance

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

func initializeProfileStorage(ctx context.Context, path string, b Binding, raw []byte, digest [32]byte, fault func(string) error) (err error) {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil || parent != filepath.Dir(path) {
		return ErrUnavailable
	}
	if err = os.Mkdir(path, 0700); err != nil {
		return ErrUnavailable
	}
	defer func() {
		if err != nil {
			err = errors.Join(ErrUncertain, err)
		}
	}()
	root, identity, err := ownedRoot(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	if err = writeExclusive(root, "profile.lock", nil, fault); err != nil {
		return err
	}
	lock, err := acquireNamedLock(root, "profile.lock")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, releaseLock(lock)) }()
	if err = writeExclusive(root, "profile.pending", raw, fault); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = failAt(fault, "rename"); err != nil {
		return err
	}
	if err = root.Rename("profile.pending", "profile.bytes"); err != nil {
		return err
	}
	saved, _, err := readOwned(root, "profile.bytes", admission.MaximumIssuerProfile)
	defer clear(saved)
	if err != nil || !bytes.Equal(saved, raw) {
		return ErrUnavailable
	}
	if err = syncDirectory(root, fault); err != nil {
		return err
	}
	if err = writeExclusive(root, "profile.pin", profilePinBytes(b, raw, digest), fault); err != nil {
		return err
	}
	if err = syncDirectory(root, fault); err != nil {
		return err
	}
	checked := &storeState{root: root, path: path, identity: identity, files: map[string]os.FileInfo{}}
	for _, name := range profileFiles {
		saved, info, e := readOwned(root, name, admission.MaximumIssuerProfile)
		if e != nil {
			clear(saved)
			return e
		}
		valid := name == "profile.lock" && len(saved) == 0 || name == "profile.bytes" && bytes.Equal(saved, raw) || name == "profile.pin" && bytes.Equal(saved, profilePinBytes(b, raw, digest))
		clear(saved)
		if !valid {
			return ErrUnavailable
		}
		checked.files[name] = info
	}
	if err = checked.checkIdentity(); err != nil {
		return err
	}
	after, err := os.Lstat(path)
	if err != nil || !privateFile(after, true) || !os.SameFile(identity, after) {
		return ErrUnavailable
	}
	return syncParent(path)
}
func openProfileStorage(ctx context.Context, path string, keys Store, v Inventory, fault func(string) error) (_ ProfileStore, err error) {
	root, identity, err := ownedRoot(path)
	if err != nil {
		return ProfileStore{}, err
	}
	files := &storeState{root: root, path: path, identity: identity, files: map[string]os.FileInfo{}}
	defer func() {
		if err != nil {
			err = errors.Join(err, releaseLock(files.lock), root.Close())
		}
	}()
	for _, n := range profileFiles {
		info, e := root.Lstat(n)
		if e != nil || !privateFile(info, false) {
			return ProfileStore{}, ErrUnavailable
		}
		files.files[n] = info
	}
	if err = files.checkIdentity(); err != nil {
		return ProfileStore{}, err
	}
	files.lock, err = acquireNamedLock(root, "profile.lock")
	if err != nil {
		return ProfileStore{}, err
	}
	raw, _, err := readOwned(root, "profile.bytes", admission.MaximumIssuerProfile)
	if err != nil {
		return ProfileStore{}, err
	}
	if err = validateProfile(raw, v); err != nil {
		return ProfileStore{}, err
	}
	pin, _, err := readOwned(root, "profile.pin", 4096)
	if err != nil || !bytes.Equal(pin, profilePinBytes(v.Binding, raw, v.Digest)) {
		return ProfileStore{}, ErrUnavailable
	}
	for _, n := range profileFiles {
		f, e := root.OpenFile(n, os.O_RDWR, 0)
		if e != nil {
			return ProfileStore{}, ErrUnavailable
		}
		info, e := f.Stat()
		syncErr := failAt(fault, "open-sync")
		if syncErr == nil {
			syncErr = f.Sync()
		}
		if e = errors.Join(e, syncErr, f.Close()); e != nil || !sameIdentity(files.files[n], info) {
			return ProfileStore{}, ErrUnavailable
		}
	}
	s := &profileState{files: files, keys: keys, raw: raw}
	if err = errors.Join(syncDirectory(root, fault), syncParent(path), s.check(), ctx.Err()); err != nil {
		return ProfileStore{}, err
	}
	return ProfileStore{s}, nil
}
