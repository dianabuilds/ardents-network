package generation

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"syscall"
)

type fileObservation struct {
	file     *os.File
	complete bool
	identity os.FileInfo
	body     []byte
	gid      uint32
	mode     os.FileMode
}

func writeFile(ctx context.Context, root *os.Root, name string, original fileObservation, body []byte, mode os.FileMode, gid uint32) (result fileObservation, returnedErr error) {
	result = original
	if err := errors.Join(ctx.Err(), observeFile(root, name, original)); err != nil {
		return result, err
	}
	file := original.file
	written, err := file.Write(body)
	if err != nil || written != len(body) {
		return result, errors.Join(io.ErrShortWrite, err)
	}
	if err := file.Chown(0, int(gid)); err != nil {
		return result, err
	}
	if err := file.Chmod(mode); err != nil {
		return result, err
	}
	if err := file.Sync(); err != nil {
		return result, err
	}
	info, err := file.Stat()
	if err != nil || !os.SameFile(original.identity, info) {
		return result, errors.Join(ErrBinding, err)
	}
	result = fileObservation{file: file, identity: info, body: bytes.Clone(body), gid: gid, mode: mode, complete: true}
	// Retain the same original inode read-only once mutation is complete. A
	// writable executable descriptor would make the later real exec fail with
	// ETXTBSY even after its mode and bytes were sealed. Open and match the
	// reader before closing the writer; failure still retains original custody.
	reader, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return result, err
	}
	readInfo, err := reader.Stat()
	if err != nil || !fileObservationMatches(result, readInfo) {
		return result, errors.Join(ErrBinding, err, reader.Close())
	}
	result.file = reader
	closeErr := file.Close()
	return result, errors.Join(closeErr, observeFile(root, name, result), ctx.Err())
}

func observeFile(root *os.Root, name string, expected fileObservation) (returnedErr error) {
	if expected.file != nil {
		info, err := expected.file.Stat()
		if err != nil || !fileObservationMatches(expected, info) {
			return errors.Join(ErrBinding, err)
		}
	}
	before, err := root.Lstat(name)
	if err != nil || !fileObservationMatches(expected, before) {
		return errors.Join(ErrBinding, err)
	}
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	info, err := file.Stat()
	if err != nil || !fileObservationMatches(expected, info) {
		return errors.Join(ErrBinding, err)
	}
	body, err := io.ReadAll(io.LimitReader(file, int64(len(expected.body))+1))
	if err != nil || !bytes.Equal(body, expected.body) {
		return errors.Join(ErrBinding, err)
	}
	final, err := root.Lstat(name)
	finalHandle, handleErr := file.Stat()
	if err != nil || handleErr != nil || !fileObservationMatches(expected, final) || !fileObservationMatches(expected, finalHandle) {
		return errors.Join(ErrBinding, err, handleErr)
	}
	return nil
}

func fileObservationMatches(expected fileObservation, info os.FileInfo) bool {
	if expected.identity == nil || info == nil || !info.Mode().IsRegular() || !os.SameFile(expected.identity, info) ||
		info.Mode() != expected.mode || info.Size() != int64(len(expected.body)) || !info.ModTime().Equal(expected.identity.ModTime()) {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	original, originalOK := expected.identity.Sys().(*syscall.Stat_t)
	return ok && originalOK && native.Uid == 0 && native.Gid == expected.gid && native.Nlink == 1 && native.Ctim == original.Ctim
}

func ownedFile(info os.FileInfo) bool {
	if info == nil || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == 0 && native.Nlink == 1
}

func fileMode(name string) (os.FileMode, bool) {
	for _, allowed := range Names() {
		if name == allowed {
			if name == "ardents-linux-amd64" || name == "ardents-text-linux-amd64" {
				return 0555, true
			}
			return 0640, true
		}
	}
	return 0, false
}
