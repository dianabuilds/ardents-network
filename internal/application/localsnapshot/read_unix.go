//go:build linux || darwin

package localsnapshot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/durablefile"
)

const snapshotChunkBytes = 16 << 10

var errSnapshotFile = errors.New("document snapshot input is unavailable or unstable")

// Read imports one explicitly selected local file in the trusted
// owner's process. Every path component is opened without following symlinks.
// Only a bounded regular file is read, and its identity, size and change
// timestamps must remain stable. The returned copy is caller-owned and must be
// cleared after transfer. No path or filesystem handle goes to the worker.
func Read(ctx context.Context, path string) ([]byte, error) {
	var output *bytes.Buffer
	err := CopyVerified(ctx, path, document.MaximumBytes, func(size int64) (io.Writer, error) {
		output = bytes.NewBuffer(make([]byte, 0, int(size)))
		return output, nil
	})
	if err != nil {
		if output != nil {
			clear(output.Bytes())
		}
		return nil, err
	}
	return output.Bytes(), nil
}

// CopyVerified transfers a selected regular file to the supplied writer after
// the caller admits its observed size. Output remains provisional until this
// call succeeds, including comparison against a second read and file close.
func CopyVerified(ctx context.Context, path string, maximum int64, destination func(int64) (io.Writer, error)) (resultErr error) {
	if ctx == nil || destination == nil || !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" {
		return errSnapshotFile
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := openSnapshotFile(path)
	if err != nil {
		return errSnapshotFile
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close(), ctx.Err()) }()
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() < 0 || before.Size() > maximum {
		return errSnapshotFile
	}
	output, err := destination(before.Size())
	if err != nil {
		return err
	}
	if output == nil {
		return errSnapshotFile
	}
	digest := sha256.New()
	if err := copySnapshotPass(ctx, file, io.MultiWriter(output, digest), before.Size()); err != nil {
		return err
	}
	return verifySnapshotFile(ctx, file, before, digest.Sum(nil))
}

func openSnapshotFile(path string) (*os.File, error) {
	parent, err := syscall.Open("/", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for _, part := range parts[:len(parts)-1] {
		next, openErr := durablefile.Openat(parent, part, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
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
	fd, openErr := durablefile.Openat(parent, parts[len(parts)-1], syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	closeErr := syscall.Close(parent)
	if openErr != nil {
		return nil, openErr
	}
	if closeErr != nil {
		_ = syscall.Close(fd)
		return nil, closeErr
	}
	return os.NewFile(uintptr(fd), "selected document snapshot"), nil
}

// copySnapshotPass copies exactly the observed size in fixed-size chunks.
// The caller must discard output if the pass or later verification fails.
func copySnapshotPass(ctx context.Context, input io.Reader, output io.Writer, size int64) error {
	var scratch [snapshotChunkBytes]byte
	defer clear(scratch[:])
	for remaining := size; remaining > 0; {
		if err := ctx.Err(); err != nil {
			return err
		}
		chunk := scratch[:min(int64(len(scratch)), remaining)]
		if _, err := io.ReadFull(input, chunk); err != nil {
			return errSnapshotFile
		}
		n, err := output.Write(chunk)
		if err != nil {
			return err
		}
		if n != len(chunk) {
			return io.ErrShortWrite
		}
		remaining -= int64(n)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if n, err := input.Read(scratch[:1]); n != 0 || err != io.EOF {
		return errSnapshotFile
	}
	return ctx.Err()
}

// A second read detects content changes even when the metadata is unchanged.
func verifySnapshotFile(ctx context.Context, file *os.File, before os.FileInfo, firstDigest []byte) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return errSnapshotFile
	}
	digest := sha256.New()
	if err := copySnapshotPass(ctx, file, digest, before.Size()); err != nil {
		return err
	}
	after, err := file.Stat()
	if err != nil || !durablefile.SameFileIdentity(before, after) || !bytes.Equal(firstDigest, digest.Sum(nil)) {
		return errSnapshotFile
	}
	return ctx.Err()
}
