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
	return retainReadOnlyFile(ctx, root, name, result)
}

// Both creation and recovered prefix repair hand off a completed mutation to
// same-inode read custody before any later exec. Open and match the reader
// before closing the writer; a failed handoff retains its original custody.
func retainReadOnlyFile(ctx context.Context, root *os.Root, name string, original fileObservation) (result fileObservation, returnedErr error) {
	result = original
	reader, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return result, err
	}
	readInfo, err := reader.Stat()
	if err != nil || !fileObservationMatches(result, readInfo) {
		return result, errors.Join(ErrBinding, err, reader.Close())
	}
	result.file = reader
	closeErr := original.file.Close()
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
	// Reobserve every original byte through a private erased scratch instead of
	// a program-sized ReadAll allocation per observation (audit IE-8, matching
	// observeSourceSelectionBytes).
	var scratch [32 << 10]byte
	defer clear(scratch[:])
	for offset := 0; offset < len(expected.body); {
		end := offset + min(len(scratch), len(expected.body)-offset)
		raw := scratch[:end-offset]
		n, err := file.ReadAt(raw, int64(offset))
		if err != nil || n != len(raw) || !bytes.Equal(raw, expected.body[offset:end]) {
			return errors.Join(ErrBinding, err)
		}
		offset = end
	}
	// Appended bytes must fail even though the observed size matched above.
	if n, err := file.ReadAt(scratch[:1], int64(len(expected.body))); n != 0 || err != io.EOF {
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
