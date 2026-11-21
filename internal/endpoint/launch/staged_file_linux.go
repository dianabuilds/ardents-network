package launch

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"syscall"
)

func writeStagedFile(ctx context.Context, root *os.Root, name string, body []byte, mode os.FileMode, gid uint32) (result fileObservation, returnedErr error) {
	if err := ctx.Err(); err != nil {
		return fileObservation{}, err
	}
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return fileObservation{}, err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	birth, err := file.Stat()
	if err != nil || !rootOwnedFile(birth) || birth.Mode().Perm() != 0600 || birth.Size() != 0 {
		return fileObservation{}, errors.Join(ErrBinding, err)
	}
	written, err := file.Write(body)
	if err != nil || written != len(body) {
		return fileObservation{}, errors.Join(io.ErrShortWrite, err)
	}
	if err := file.Chown(0, int(gid)); err != nil {
		return fileObservation{}, err
	}
	if err := file.Chmod(mode); err != nil {
		return fileObservation{}, err
	}
	if err := file.Sync(); err != nil {
		return fileObservation{}, err
	}
	info, err := file.Stat()
	if err != nil || !os.SameFile(birth, info) {
		return fileObservation{}, errors.Join(ErrBinding, err)
	}
	result = fileObservation{identity: info, body: bytes.Clone(body), gid: gid, mode: mode}
	return result, errors.Join(observeStagedFile(root, name, result), ctx.Err())
}

func observeStagedFile(root *os.Root, name string, expected fileObservation) (returnedErr error) {
	before, err := root.Lstat(name)
	if err != nil || !observedFileMatches(expected, before) {
		return errors.Join(ErrBinding, err)
	}
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	info, err := file.Stat()
	if err != nil || !observedFileMatches(expected, info) {
		return errors.Join(ErrBinding, err)
	}
	body, err := io.ReadAll(io.LimitReader(file, int64(len(expected.body))+1))
	if err != nil || !bytes.Equal(body, expected.body) {
		return errors.Join(ErrBinding, err)
	}
	final, err := root.Lstat(name)
	finalHandle, handleErr := file.Stat()
	if err != nil || handleErr != nil || !observedFileMatches(expected, final) || !observedFileMatches(expected, finalHandle) {
		return errors.Join(ErrBinding, err, handleErr)
	}
	return nil
}
