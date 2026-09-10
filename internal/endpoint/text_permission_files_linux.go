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
)

// The trusted Endpoint composition supplies these owner-only paths. Neither
// Application transport accepts a path, permission, holder key or context ID.
// Export returns the public commitment separately for explicit Custody approval.
func (owner *textContext) exportTextPermissionFile(ctx context.Context, path string, maxima [3]uint32) (commitment [32]byte, outcome error) {
	if ctx == nil || ctx.Err() != nil {
		return [32]byte{}, errors.New("text permission export canceled")
	}
	public, digest, err := owner.requestTextPermission(maxima)
	if err != nil {
		return [32]byte{}, err
	}
	defer clear(public)
	root, name, err := openTextPermissionDirectory(path)
	if err != nil {
		return [32]byte{}, err
	}
	defer func() { outcome = errors.Join(outcome, root.Close()) }()
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if errors.Is(err, os.ErrExist) {
		existing, readErr := readTextPermissionFile(root, name, 512)
		defer clear(existing)
		if readErr != nil || !bytes.Equal(existing, public) {
			return [32]byte{}, errors.New("text permission request destination conflicts")
		}
	} else if err != nil {
		return [32]byte{}, errors.New("text permission request destination unavailable")
	} else {
		written, writeErr := file.Write(public)
		if written != len(public) {
			writeErr = errors.Join(writeErr, io.ErrShortWrite)
		}
		syncErr := file.Sync()
		closeErr := file.Close()
		if errors.Join(writeErr, syncErr, closeErr) != nil {
			return [32]byte{}, errors.New("text permission request persistence failed")
		}

	}
	// Exact retries must finish persistence too: a failed earlier fsync can
	// leave complete matching bytes without a durable acknowledgement.
	if err := syncTextPermissionRequest(root, name); err != nil {
		return [32]byte{}, err
	}
	// Exporting public bytes grants nothing. Recheck without generating another
	// holder if the hour changed while the file operation was in progress.
	if err := ctx.Err(); err != nil {
		return [32]byte{}, err
	}
	owner.mu.Lock()
	profile, now, err := owner.textPermissionProfileLocked()
	pending := owner.permission
	current := pending != nil && pending.digest == digest && pending.profile == profile &&
		!now.Before(pending.request.Permission.NotBefore) && now.Before(pending.request.Permission.NotAfter)
	owner.mu.Unlock()
	if err != nil || !current {
		return [32]byte{}, errors.New("text permission export context changed")
	}
	return digest, nil
}

// Import consumes the existing Custody permission format and the separately
// retained request digest. Files are transport, never authority or context
// restoration: the in-memory owner performs all currentness checks again.
func (owner *textContext) importTextPermissionFile(ctx context.Context, path string, digest [32]byte) (outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return errors.New("text permission import canceled")
	}
	owner.mu.Lock()
	_, _, err := owner.textPermissionProfileLocked()
	matches := owner.permission != nil && owner.permission.digest == digest && digest != [32]byte{}
	owner.mu.Unlock()
	if err != nil || !matches {
		return errors.New("text permission import has no live request")
	}
	root, name, err := openTextPermissionDirectory(path)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, root.Close()) }()
	raw, err := readTextPermissionFile(root, name, 228)
	if err != nil {
		return err
	}
	defer clear(raw)
	if len(raw) != 228 {
		return errors.New("text permission response length is invalid")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return owner.importTextPermission(digest, raw)
}

func openTextPermissionDirectory(path string) (*os.Root, string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, "", errors.New("text permission path is not absolute and canonical")
	}
	parent, name := filepath.Dir(path), filepath.Base(path)
	before, err := os.Lstat(parent)
	if err != nil || !privateTextPermissionFile(before, true) {
		return nil, "", errors.New("text permission directory is not owner-private")
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return nil, "", errors.New("text permission directory unavailable")
	}
	after, err := root.Stat(".")
	if err != nil || !privateTextPermissionFile(after, true) || !os.SameFile(before, after) {
		return nil, "", errors.Join(errors.New("text permission directory changed"), root.Close())
	}
	return root, name, nil
}

func privateTextPermissionFile(info os.FileInfo, directory bool) bool {
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

func readTextPermissionFile(root *os.Root, name string, maximum int64) ([]byte, error) {
	before, err := root.Lstat(name)
	if err != nil || !privateTextPermissionFile(before, false) || before.Size() <= 0 || before.Size() > maximum {
		return nil, errors.New("text permission file is not bounded and owner-private")
	}
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.New("text permission file unavailable")
	}
	opened, statErr := file.Stat()
	if statErr != nil || !privateTextPermissionFile(opened, false) || !os.SameFile(before, opened) {
		return nil, errors.Join(errors.New("text permission file changed before read"), file.Close())
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
	after, statErr := file.Stat()
	named, nameErr := root.Lstat(name)
	closeErr := file.Close()
	if errors.Join(readErr, statErr, nameErr, closeErr) != nil || int64(len(raw)) != before.Size() ||
		!privateTextPermissionFile(after, false) || !privateTextPermissionFile(named, false) ||
		!os.SameFile(before, after) || !os.SameFile(before, named) || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		clear(raw)
		return nil, errors.New("text permission file changed during read")
	}
	return raw, nil
}

func syncTextPermissionRequest(root *os.Root, name string) error {
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return errors.New("text permission request persistence unavailable")
	}
	info, err := file.Stat()
	if err != nil || !privateTextPermissionFile(info, false) || info.Size() <= 0 || info.Size() > 512 {
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
