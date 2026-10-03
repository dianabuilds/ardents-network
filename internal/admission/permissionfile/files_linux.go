//go:build linux

package permissionfile

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// Path pins the private directory for a public request or approved response.
// It grants no authority to accept the response bytes.
type Path struct {
	root *os.Root
	name string
}

// Open verifies and pins the owner-private directory for one canonical path.
// The caller must Close it after all context checks and file effects.
func Open(path string) (*Path, error) {
	root, name, err := openDirectory(path)
	if err != nil {
		return nil, err
	}
	return &Path{root: root, name: name}, nil
}

// Present reports whether the selected path has appeared. ReadResponse reopens
// and verifies the file identity before Endpoint interprets the bytes.
func (path *Path) Present() (bool, error) {
	if path == nil || path.root == nil {
		return false, errors.New("text permission response path unavailable")
	}
	_, err := path.root.Lstat(path.name)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, errors.New("text permission response path unavailable")
}

func (path *Path) Close() error {
	if path == nil || path.root == nil {
		return nil
	}
	err := path.root.Close()
	path.root = nil
	return err
}

// PublishRequest writes one exact public request to a new owner-private file,
// or accepts an exact existing file, then syncs both file and directory.
func (path *Path) PublishRequest(public []byte) error {
	if path == nil || path.root == nil {
		return errors.New("text permission request destination unavailable")
	}
	file, err := path.root.OpenFile(path.name, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if errors.Is(err, os.ErrExist) {
		existing, readErr := readFile(path.root, path.name, 512)
		defer clear(existing)
		if readErr != nil || !bytes.Equal(existing, public) {
			return errors.New("text permission request destination conflicts")
		}
	} else if err != nil {
		return errors.New("text permission request destination unavailable")
	} else {
		written, writeErr := file.Write(public)
		if written != len(public) {
			writeErr = errors.Join(writeErr, io.ErrShortWrite)
		}
		syncErr := file.Sync()
		closeErr := file.Close()
		if errors.Join(writeErr, syncErr, closeErr) != nil {
			return errors.New("text permission request persistence failed")
		}
	}
	return syncRequest(path.root, path.name)
}

// ReadResponse reads one exact fixed-size owner-private approval file. It does
// not authenticate the permission or restore a participant context.
func (path *Path) ReadResponse() ([]byte, error) {
	if path == nil || path.root == nil {
		return nil, errors.New("text permission file unavailable")
	}
	raw, err := readFile(path.root, path.name, 228)
	if err != nil {
		return nil, err
	}
	if len(raw) != 228 {
		clear(raw)
		return nil, errors.New("text permission response length is invalid")
	}
	return raw, nil
}

func openDirectory(path string) (*os.Root, string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, "", errors.New("text permission path is not absolute and canonical")
	}
	parent, name := filepath.Dir(path), filepath.Base(path)
	before, err := os.Lstat(parent)
	if err != nil || !privateFile(before, true) {
		return nil, "", errors.New("text permission directory is not owner-private")
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return nil, "", errors.New("text permission directory unavailable")
	}
	after, err := root.Stat(".")
	if err != nil || !privateFile(after, true) || !os.SameFile(before, after) {
		return nil, "", errors.Join(errors.New("text permission directory changed"), root.Close())
	}
	return root, name, nil
}

func privateFile(info os.FileInfo, directory bool) bool {
	if info == nil {
		return false
	}
	identity, ok := info.Sys().(*syscall.Stat_t)
	if !ok || identity.Uid != uint32(os.Geteuid()) {
		return false
	}
	if directory {
		return info.IsDir() && info.Mode().Perm() == 0700
	}
	return info.Mode().IsRegular() && info.Mode().Perm() == 0600 && identity.Nlink == 1
}

func readFile(root *os.Root, name string, maximum int64) ([]byte, error) {
	before, err := root.Lstat(name)
	if err != nil || !privateFile(before, false) || before.Size() <= 0 || before.Size() > maximum {
		return nil, errors.New("text permission file is not bounded and owner-private")
	}
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.New("text permission file unavailable")
	}
	opened, statErr := file.Stat()
	if statErr != nil || !privateFile(opened, false) || !os.SameFile(before, opened) {
		return nil, errors.Join(errors.New("text permission file changed before read"), file.Close())
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
	after, statErr := file.Stat()
	named, nameErr := root.Lstat(name)
	closeErr := file.Close()
	if errors.Join(readErr, statErr, nameErr, closeErr) != nil || int64(len(raw)) != before.Size() ||
		!privateFile(after, false) || !privateFile(named, false) ||
		!os.SameFile(before, after) || !os.SameFile(before, named) || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		clear(raw)
		return nil, errors.New("text permission file changed during read")
	}
	return raw, nil
}

func syncRequest(root *os.Root, name string) error {
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return errors.New("text permission request persistence unavailable")
	}
	info, err := file.Stat()
	if err != nil || !privateFile(info, false) || info.Size() <= 0 || info.Size() > 512 {
		return errors.Join(errors.New("text permission request changed before persistence"), file.Close())
	}
	if errors.Join(file.Sync(), file.Close()) != nil {
		return errors.New("text permission request persistence failed")
	}
	directory, err := root.Open(".")
	if err != nil {
		return errors.New("text permission request directory unavailable")
	}
	if errors.Join(directory.Sync(), directory.Close()) != nil {
		return errors.New("text permission request directory persistence failed")
	}
	return nil
}
