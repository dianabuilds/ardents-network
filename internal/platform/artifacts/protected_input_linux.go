//go:build linux

package artifacts

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/durablefile"
)

// ProtectedInput retains the original file and every direct ancestor. A later
// lookup compares with this inode; it cannot adopt a newly supplied input.
type ProtectedInput struct {
	parent  *DirectoryChain
	file    *os.File
	info    os.FileInfo
	raw     []byte
	private bool
	failure error
}

// ProtectedFileMode admits an original root-owned regular file or an exact 0600 secret.
func ProtectedFileMode(info os.FileInfo, private bool) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == 0 && stat.Nlink == 1 && info.Mode().IsRegular() &&
		info.Mode() == info.Mode().Perm() && info.Mode().Perm()&0022 == 0 && (!private || info.Mode() == 0600)
}

// OpenProtectedInput joins root-controlled ancestors with one retained bounded file.
func OpenProtectedInput(ctx context.Context, path string, maximum int64, private bool) (input *ProtectedInput, err error) {
	if ctx == nil {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p, err := OpenDirectoryChain(ctx, filepath.Dir(path), nil)
	if err != nil {
		return nil, err
	}
	input = &ProtectedInput{parent: p, private: private}
	defer func() {
		if err != nil {
			err = errors.Join(err, input.Close())
			input = nil
		}
	}()
	if err = errors.Join(ctx.Err(), p.CheckRootAccess()); err != nil {
		return input, err
	}
	input.file, err = p.OpenFile(filepath.Base(path))
	if err != nil {
		return input, err
	}
	input.info, err = input.file.Stat()
	if err != nil || !ProtectedFileMode(input.info, private) || input.info.Size() < 1 || input.info.Size() > maximum {
		return input, errors.Join(ErrInput, err)
	}
	input.raw, err = io.ReadAll(io.LimitReader(input.file, maximum+1))
	if err != nil || int64(len(input.raw)) > maximum {
		return input, errors.Join(ErrInput, err)
	}
	return input, input.Check(ctx)
}

// Close releases the original descriptor and ancestors and clears sensitive bytes.
func (s *ProtectedInput) Close() (err error) {
	// Physical reads are synchronous and joined before owned copies are erased.
	if s.private {
		clear(s.raw)
	}
	if s.file != nil {
		err = s.file.Close()
	}
	return errors.Join(err, s.parent.Close())
}

// Check reobserves the original bytes and retains the first failure.
func (s *ProtectedInput) Check(ctx context.Context) (err error) {
	if s.failure != nil {
		return s.failure
	}
	defer func() { s.failure = err }()
	if err := s.CheckIdentity(ctx); err != nil {
		return err
	}
	// Re-read only the retained original descriptor. Detect byte changes even
	// when an adversarial writer restores the original size and timestamps.
	saved := make([]byte, len(s.raw)+1)
	if s.private {
		defer clear(saved)
	}
	n, readErr := s.file.ReadAt(saved, 0)
	if readErr != nil && readErr != io.EOF {
		return readErr
	}
	if n != len(s.raw) || !bytes.Equal(saved[:n], s.raw) {
		return ErrInput
	}
	after, err := s.file.Stat()
	if err != nil {
		return err
	}
	if !durablefile.SameFileIdentity(s.info, after) {
		return ErrInput
	}
	return ctx.Err()
}

// CheckIdentity reobserves descriptors, links and access without rereading bytes.
func (s *ProtectedInput) CheckIdentity(ctx context.Context) error {
	if err := errors.Join(ctx.Err(), s.parent.CheckRootAccess()); err != nil {
		return err
	}
	info, err := s.file.Stat()
	if err != nil || !durablefile.SameFileIdentity(s.info, info) {
		return errors.Join(ErrInput, err)
	}
	return s.parent.CheckRetainedFileLink(s.file.Name(), s.info)
}

// Bytes borrows the unchanged original contents until Close.
func (s *ProtectedInput) Bytes() []byte {
	return s.raw
}

// Descriptor borrows the original file without taking ownership.
func (s *ProtectedInput) Descriptor() *os.File {
	return s.file
}

// Info reports the original admitted metadata.
func (s *ProtectedInput) Info() os.FileInfo {
	return s.info
}

// ErrNotExt4 reports refusal of a descriptor without native ext4 storage.
var ErrNotExt4 = errors.New("original artifact requires native ext4")

// RequireExt4 checks the filesystem of an already owned original descriptor.
func RequireExt4(file *os.File) error {
	var filesystem syscall.Statfs_t
	if err := syscall.Fstatfs(int(file.Fd()), &filesystem); err != nil || filesystem.Type != 0xef53 {
		return errors.Join(ErrNotExt4, err)
	}
	return nil
}
