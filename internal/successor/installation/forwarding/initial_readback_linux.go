package forwarding

import (
	"context"
	"errors"
	"io"
	"os"
	"syscall"
)

// The creator retains its original exclusive journal and all ancestor/native
// custody across this separate read-only readback. No writer descriptor, cached
// record or born map supplies the observations below. This is not a standalone
// startup opener: that future consumer must acquire its own completed lease,
// ancestors, principal, fresh Release and native invocation admission.
func verifyInitialBirthReadback(ctx context.Context, journal string, b binding, hashes map[string]string) error {
	raw, originalJournal, err := readInitialOriginal(ctx, journal, 4<<20, 0600, 0, 0, false)
	if err != nil {
		return err
	}
	records, err := decodeInitialRecords(raw, b, hashes)
	if err != nil {
		return err
	}
	for path, record := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, file := hashes[path]
		directory := path != journal && !file
		body, info, err := readInitialOriginal(ctx, path, 64<<20, os.FileMode(record.Mode), record.UID, record.GID, directory)
		if err != nil {
			return err
		}
		native := info.Sys().(*syscall.Stat_t)
		matches := uint64(native.Dev) == record.Device && native.Ino == record.Inode && (!file || digest(body) == hashes[path])
		if path == journal {
			matches = matches && sameInputFile(originalJournal, info) && digest(body) == digest(raw)
		}
		clear(body)
		if !matches {
			return ErrBinding
		}
	}
	return ctx.Err()
}

func readInitialOriginal(ctx context.Context, path string, limit int64, mode os.FileMode, uid, gid uint32, directory bool) (raw []byte, info os.FileInfo, returned error) {
	if ctx == nil || os.Geteuid() != 0 || !validPath(path) {
		return nil, nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	before, err := os.Lstat(path)
	if err != nil || !nodeAccess(before, mode, uid, gid, directory) {
		return nil, nil, errors.Join(ErrBinding, err)
	}
	flags := os.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK
	if directory {
		flags |= syscall.O_DIRECTORY
	}
	file, err := os.OpenFile(path, flags, 0)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		returned = errors.Join(returned, file.Close(), ctx.Err())
		if returned != nil {
			clear(raw)
			raw, info = nil, nil
		}
	}()
	info, err = file.Stat()
	if err != nil || !os.SameFile(before, info) || !nodeAccess(info, mode, uid, gid, directory) || (!directory && !sameInputFile(before, info)) {
		return nil, nil, errors.Join(ErrBinding, err)
	}
	if !directory {
		if info.Size() < 0 || info.Size() > limit {
			return nil, nil, ErrBinding
		}
		raw, err = io.ReadAll(io.LimitReader(file, limit+1))
		if err != nil || len(raw) != int(info.Size()) {
			return raw, nil, errors.Join(ErrBinding, err)
		}
	}
	after, statErr := file.Stat()
	named, nameErr := os.Lstat(path)
	if !os.SameFile(info, after) || !os.SameFile(info, named) || !nodeAccess(after, mode, uid, gid, directory) || !nodeAccess(named, mode, uid, gid, directory) || (!directory && (!sameInputFile(info, after) || !sameInputFile(info, named))) {
		return raw, nil, errors.Join(ErrBinding, statErr, nameErr)
	}
	return raw, info, errors.Join(statErr, nameErr, ctx.Err())
}
