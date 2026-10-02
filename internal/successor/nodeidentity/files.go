package nodeidentity

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
)

var retainedFiles = []string{"identity.pin", "identity.key", "identity.lock"}

func ownedRoot(path string) (*os.Root, os.FileInfo, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, nil, ErrInvalid
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || resolved != path {
		return nil, nil, ErrUnavailable
	}
	before, err := os.Lstat(path)
	if err != nil || !privateFile(before, true) {
		return nil, nil, ErrUnavailable
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	after, err := root.Stat(".")
	if err != nil || !os.SameFile(before, after) {
		return nil, nil, errors.Join(ErrUnavailable, root.Close())
	}
	return root, before, nil
}
func sameIdentity(a, b os.FileInfo) bool {
	return a != nil && b != nil && os.SameFile(a, b) && privateFile(b, false) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}
func readOwned(root *os.Root, name string, limit int64) (_ []byte, _ os.FileInfo, err error) {
	before, err := root.Lstat(name)
	if err != nil || !privateFile(before, false) || before.Size() > limit {
		return nil, nil, ErrUnavailable
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	opened, err := f.Stat()
	if err != nil || !sameIdentity(before, opened) {
		return nil, nil, ErrUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(raw)) > limit {
		clear(raw)
		return nil, nil, ErrUnavailable
	}
	after, err := root.Lstat(name)
	if err != nil || !sameIdentity(opened, after) {
		clear(raw)
		return nil, nil, ErrUnavailable
	}
	return raw, after, nil
}
func failAt(fault func(string) error, phase string) error {
	if fault != nil {
		return fault(phase)
	}
	return nil
}
func writeExclusive(root *os.Root, name string, raw []byte, fault func(string) error) error {
	f, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	writeErr := failAt(fault, "write")
	if writeErr == nil {
		n, e := f.Write(raw)
		writeErr = e
		if n != len(raw) && e == nil {
			writeErr = io.ErrShortWrite
		}
	}
	syncErr := failAt(fault, "sync")
	if syncErr == nil {
		syncErr = f.Sync()
	}
	return errors.Join(writeErr, syncErr, f.Close(), failAt(fault, "close"))
}
func syncDirectory(root *os.Root, fault func(string) error) error {
	f, e := root.Open(".")
	if e != nil {
		return e
	}
	syncErr := failAt(fault, "directory-sync")
	if syncErr == nil {
		syncErr = f.Sync()
	}
	return errors.Join(syncErr, f.Close())
}
func syncParent(path string) error {
	f, e := os.Open(filepath.Dir(path))
	if e != nil {
		return e
	}
	return errors.Join(f.Sync(), f.Close())
}
func (s *identityState) check() error {
	current, e := os.Lstat(s.path)
	if e != nil || !privateFile(current, true) || !os.SameFile(s.identity, current) {
		return ErrUnavailable
	}
	resolved, e := filepath.EvalSymlinks(s.path)
	if e != nil || resolved != s.path {
		return ErrUnavailable
	}
	d, e := s.root.Open(".")
	if e != nil {
		return ErrUnavailable
	}
	names, e := d.Readdirnames(-1)
	closeErr := d.Close()
	if e != nil || closeErr != nil || len(names) != 3 {
		return ErrUnavailable
	}
	for _, n := range names {
		before, ok := s.files[n]
		after, e := s.root.Lstat(n)
		if !ok || e != nil || !sameIdentity(before, after) {
			return ErrUnavailable
		}
	}
	return nil
}
func initialize(ctx context.Context, path string, b Binding, raw []byte, fault func(string) error) (err error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return ErrInvalid
	}
	parent, e := filepath.EvalSymlinks(filepath.Dir(path))
	if e != nil || parent != filepath.Dir(path) {
		return ErrUnavailable
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if e = os.Mkdir(path, 0700); e != nil {
		return ErrUnavailable
	}
	defer func() {
		if err != nil {
			err = errors.Join(ErrUncertain, err)
		}
	}()
	root, identity, e := ownedRoot(path)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	if e = writeExclusive(root, "identity.lock", nil, fault); e != nil {
		return e
	}
	lock, e := acquireLock(root)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, releaseLock(lock)) }()
	if e = writeExclusive(root, "identity.pending", raw, fault); e != nil {
		return e
	}
	if e = failAt(fault, "rename"); e != nil {
		return e
	}
	if e = root.Rename("identity.pending", "identity.key"); e != nil {
		return e
	}
	saved, _, e := readOwned(root, "identity.key", 4096)
	defer clear(saved)
	if e != nil || !bytes.Equal(saved, raw) {
		return ErrUnavailable
	}
	if e = syncDirectory(root, fault); e != nil {
		return e
	}
	if e = writeExclusive(root, "identity.pin", pinBytes(b, raw), fault); e != nil {
		return e
	}
	if e = syncDirectory(root, fault); e != nil {
		return e
	}
	checked := &identityState{root: root, path: path, identity: identity, files: map[string]os.FileInfo{}}
	for _, name := range retainedFiles {
		data, info, e := readOwned(root, name, 4096)
		valid := name == "identity.lock" && len(data) == 0 || name == "identity.key" && bytes.Equal(data, raw) || name == "identity.pin" && bytes.Equal(data, pinBytes(b, raw))
		clear(data)
		if e != nil || !valid {
			return ErrUnavailable
		}
		checked.files[name] = info
	}
	return errors.Join(checked.check(), syncParent(path))
}
func openStore(ctx context.Context, path string, b Binding, fault func(string) error) (_ Store, err error) {
	root, identity, err := ownedRoot(path)
	if err != nil {
		return Store{}, err
	}
	s := &identityState{root: root, path: path, identity: identity, binding: b, files: map[string]os.FileInfo{}}
	defer func() {
		if err != nil {
			clear(s.key)
			err = errors.Join(err, releaseLock(s.lock), root.Close())
		}
	}()
	for _, n := range retainedFiles {
		info, e := root.Lstat(n)
		if e != nil || !privateFile(info, false) {
			return Store{}, ErrUnavailable
		}
		s.files[n] = info
	}
	if err = s.check(); err != nil {
		return Store{}, err
	}
	s.lock, err = acquireLock(root)
	if err != nil {
		return Store{}, err
	}
	raw, _, err := readOwned(root, "identity.key", 4096)
	defer clear(raw)
	if err != nil {
		return Store{}, err
	}
	pin, _, err := readOwned(root, "identity.pin", 4096)
	if err != nil || !bytes.Equal(pin, pinBytes(b, raw)) {
		return Store{}, ErrUnavailable
	}
	s.key, err = parseKey(raw, b)
	if err != nil {
		return Store{}, ErrUnavailable
	}
	for _, n := range retainedFiles {
		f, e := root.OpenFile(n, os.O_RDWR, 0)
		if e != nil {
			return Store{}, ErrUnavailable
		}
		info, e := f.Stat()
		syncErr := failAt(fault, "open-sync")
		if syncErr == nil {
			syncErr = f.Sync()
		}
		if e = errors.Join(e, syncErr, f.Close()); e != nil || !sameIdentity(s.files[n], info) {
			return Store{}, ErrUnavailable
		}
	}
	if err = errors.Join(syncDirectory(root, fault), syncParent(path), s.check(), ctx.Err()); err != nil {
		return Store{}, err
	}
	return Store{s}, nil
}
