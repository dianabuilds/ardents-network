//go:build linux

package textdocument

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf8"
)

var errSnapshotFile = errors.New("text snapshot input is unavailable or unstable")

// ReadSnapshotFile imports one explicitly selected local file in the trusted
// owner's process. Every path component is opened without following symlinks.
// Only a bounded regular file is read, and its identity, size and change
// timestamps must remain stable. The returned copy is caller-owned and must be
// cleared after transfer. No path or filesystem handle goes to the worker.
func ReadSnapshotFile(ctx context.Context, path string) (body []byte, resultErr error) {
	if ctx == nil || !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" {
		return nil, errSnapshotFile
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := openSnapshotFile(path)
	if err != nil {
		return nil, errSnapshotFile
	}
	defer func() {
		resultErr = errors.Join(resultErr, file.Close(), ctx.Err())
		if resultErr != nil {
			clear(body)
			body = nil
		}
	}()
	return readStableSnapshot(ctx, file)
}

func openSnapshotFile(path string) (*os.File, error) {
	parent, err := syscall.Open("/", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for _, part := range parts[:len(parts)-1] {
		next, openErr := syscall.Openat(parent, part, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		closeErr := syscall.Close(parent)
		if openErr != nil {
			return nil, openErr
		}
		if closeErr != nil {
			_ = syscall.Close(next)
			return nil, closeErr
		}
		parent = next
	}
	// NONBLOCK prevents FIFO open from waiting before the regular-file check.
	fd, openErr := syscall.Openat(parent, parts[len(parts)-1], syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	closeErr := syscall.Close(parent)
	if openErr != nil {
		return nil, openErr
	}
	if closeErr != nil {
		_ = syscall.Close(fd)
		return nil, closeErr
	}
	return os.NewFile(uintptr(fd), "selected text snapshot"), nil
}

func readStableSnapshot(ctx context.Context, file *os.File) ([]byte, error) {
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() < 0 || before.Size() > MaximumBytes {
		return nil, errSnapshotFile
	}
	body := make([]byte, int(before.Size()))
	for offset := 0; offset < len(body); {
		if err := ctx.Err(); err != nil {
			clear(body)
			return nil, err
		}
		end := min(offset+16384, len(body))
		n, err := io.ReadFull(file, body[offset:end])
		offset += n
		if err != nil {
			clear(body)
			return nil, errSnapshotFile
		}
	}
	if err := verifySnapshotFile(ctx, file, before, body); err != nil {
		clear(body)
		return nil, err
	}
	return body, nil
}

// Compare a second bounded pass as well as metadata. Filesystem change times
// can have coarse granularity; equal timestamps alone do not prove stability.
func verifySnapshotFile(ctx context.Context, file *os.File, before os.FileInfo, body []byte) error {
	var excess [1]byte
	if n, err := file.Read(excess[:]); n != 0 || err != io.EOF {
		return errSnapshotFile
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return errSnapshotFile
	}
	var scratch [16384]byte
	defer clear(scratch[:])
	for offset := 0; offset < len(body); {
		if err := ctx.Err(); err != nil {
			return err
		}
		length := min(len(scratch), len(body)-offset)
		if _, err := io.ReadFull(file, scratch[:length]); err != nil || !bytes.Equal(scratch[:length], body[offset:offset+length]) {
			return errSnapshotFile
		}
		offset += length
	}
	n, readErr := file.Read(excess[:])
	after, statErr := file.Stat()
	if n != 0 || readErr != io.EOF || statErr != nil || !stableSnapshotIdentity(before, after) || !utf8.Valid(body) {
		return errSnapshotFile
	}
	return ctx.Err()
}
func stableSnapshotIdentity(before, after os.FileInfo) bool {
	first, ok := before.Sys().(*syscall.Stat_t)
	last, otherOK := after.Sys().(*syscall.Stat_t)
	return ok && otherOK && os.SameFile(before, after) && before.Mode() == after.Mode() &&
		first.Size == last.Size && first.Mtim == last.Mtim && first.Ctim == last.Ctim &&
		first.Nlink == last.Nlink && first.Uid == last.Uid && first.Gid == last.Gid
}
