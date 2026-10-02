package issuance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const marker = "ardents-issuer-key-material-v1\n"

var retainedFiles = []string{"issuer.pin", "issuer.keys", "issuer.lock"}

func pinBytes(b Binding, raw []byte, digest [32]byte) []byte {
	pin := append([]byte(marker), bindingBytes(b)...)
	materialDigest := sha256.Sum256(raw)
	pin = append(pin, materialDigest[:]...)
	return append(pin, digest[:]...)
}

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
	closeErr := errors.Join(f.Close(), failAt(fault, "close"))
	return errors.Join(writeErr, syncErr, closeErr)
}

func syncDirectory(root *os.Root, fault func(string) error) error {
	f, err := root.Open(".")
	if err != nil {
		return err
	}
	syncErr := failAt(fault, "directory-sync")
	if syncErr == nil {
		syncErr = f.Sync()
	}
	return errors.Join(syncErr, f.Close())
}
func syncParent(path string) error {
	f, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	return errors.Join(f.Sync(), f.Close())
}

func initializeStorage(ctx context.Context, path string, b Binding, raw []byte, digest [32]byte, fault func(string) error) (err error) {
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
	if err = writeExclusive(root, "issuer.lock", nil, fault); err != nil {
		return err
	}
	lock, err := acquireLock(root)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, releaseLock(lock)) }()
	if err = writeExclusive(root, "issuer.pending", raw, fault); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = failAt(fault, "rename"); err != nil {
		return err
	}
	if err = root.Rename("issuer.pending", "issuer.keys"); err != nil {
		return err
	}
	saved, _, err := readOwned(root, "issuer.keys", maximumMaterial)
	defer clear(saved)
	if err != nil || !bytes.Equal(saved, raw) {
		return ErrUnavailable
	}
	if err = syncDirectory(root, fault); err != nil {
		return err
	}
	if err = writeExclusive(root, "issuer.pin", pinBytes(b, raw, digest), fault); err != nil {
		return err
	}
	if err = syncDirectory(root, fault); err != nil {
		return err
	}
	checked := &storeState{root: root, path: path, identity: identity, files: map[string]os.FileInfo{}}
	for _, name := range retainedFiles {
		saved, info, e := readOwned(root, name, maximumMaterial)
		if e != nil {
			clear(saved)
			return e
		}
		valid := name == "issuer.lock" && len(saved) == 0 || name == "issuer.keys" && bytes.Equal(saved, raw) || name == "issuer.pin" && bytes.Equal(saved, pinBytes(b, raw, digest))
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

func readOwned(root *os.Root, name string, limit int64) (_ []byte, _ os.FileInfo, err error) {
	before, err := root.Lstat(name)
	if err != nil || !privateFile(before, false) || before.Size() > limit {
		return nil, nil, ErrUnavailable
	}
	f, err := root.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		return nil, nil, ErrUnavailable
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) || !privateFile(opened, false) {
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
func sameIdentity(a, b os.FileInfo) bool {
	return a != nil && b != nil && os.SameFile(a, b) && privateFile(b, false) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}
func (s *storeState) checkIdentity() error {
	root, err := os.Lstat(s.path)
	if err != nil || !privateFile(root, true) || !os.SameFile(s.identity, root) {
		return ErrUnavailable
	}
	directory, err := s.root.Open(".")
	if err != nil {
		return ErrUnavailable
	}
	names, readErr := directory.Readdirnames(-1)
	closeErr := directory.Close()
	if readErr != nil || closeErr != nil || len(names) != 3 {
		return ErrUnavailable
	}
	for _, name := range names {
		if _, ok := s.files[name]; !ok {
			return ErrUnavailable
		}
	}
	for name, before := range s.files {
		after, e := s.root.Lstat(name)
		if e != nil || !sameIdentity(before, after) {
			return ErrUnavailable
		}
	}
	return nil
}

func openStorage(ctx context.Context, path string, b Binding, fault func(string) error) (_ Store, err error) {
	root, identity, err := ownedRoot(path)
	if err != nil {
		return Store{}, err
	}
	s := &storeState{root: root, path: path, identity: identity, files: map[string]os.FileInfo{}}
	defer func() {
		if err != nil {
			err = errors.Join(err, releaseLock(s.lock), root.Close())
		}
	}()
	// Validate all retained names and metadata before flock or any flush.
	directory, err := root.Open(".")
	if err != nil {
		return Store{}, ErrUnavailable
	}
	names, readErr := directory.Readdirnames(-1)
	closeErr := directory.Close()
	if readErr != nil || closeErr != nil || len(names) != 3 {
		return Store{}, ErrUnavailable
	}
	for _, name := range retainedFiles {
		info, e := root.Lstat(name)
		if e != nil || !privateFile(info, false) {
			return Store{}, ErrUnavailable
		}
		s.files[name] = info
	}
	if err = s.checkIdentity(); err != nil {
		return Store{}, err
	}
	s.lock, err = acquireLock(root)
	if err != nil {
		return Store{}, err
	}
	raw, info, err := readOwned(root, "issuer.keys", maximumMaterial)
	defer clear(raw)
	if err != nil {
		return Store{}, ErrUnavailable
	}
	s.files["issuer.keys"] = info
	s.inventory, err = decodeMaterial(raw, b)
	if err != nil {
		return Store{}, err
	}
	pin, info, err := readOwned(root, "issuer.pin", 4096)
	if err != nil || !bytes.Equal(pin, pinBytes(b, raw, s.inventory.Digest)) {
		return Store{}, ErrUnavailable
	}
	s.files["issuer.pin"] = info
	if ctx.Err() != nil {
		return Store{}, ctx.Err()
	}
	for _, name := range retainedFiles {
		f, e := root.OpenFile(name, os.O_RDWR, 0)
		if e != nil {
			return Store{}, ErrUnavailable
		}
		opened, e := f.Stat()
		syncErr := failAt(fault, "open-sync")
		if syncErr == nil {
			syncErr = f.Sync()
		}
		endErr := errors.Join(e, syncErr, f.Close())
		if endErr != nil || !sameIdentity(s.files[name], opened) {
			return Store{}, ErrUnavailable
		}
	}
	if err = errors.Join(syncDirectory(root, fault), syncParent(path), s.checkIdentity()); err != nil {
		return Store{}, ErrUnavailable
	}
	return Store{state: s}, nil
}
