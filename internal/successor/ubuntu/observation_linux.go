package ubuntu

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"syscall"
)

// ErrBinding reports that the original platform file or declaration differs.
var ErrBinding = errors.New("installation platform: binding differs")

type releaseFile struct {
	path string
	file *os.File
	info os.FileInfo
	raw  []byte
}

func readReleaseFile(path string) (result *releaseFile, err error) {
	// The caller supplies only the fixed os-release path.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	result = &releaseFile{path: path, file: f}
	defer func() {
		if err != nil {
			err = errors.Join(err, f.Close())
			clear(result.raw)
			result = nil
		}
	}()
	result.info, err = f.Stat()
	if err != nil {
		return result, err
	}
	st, ok := result.info.Sys().(*syscall.Stat_t)
	if !ok || !result.info.Mode().IsRegular() || st.Uid != 0 || st.Nlink != 1 || result.info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || result.info.Mode().Perm()&0022 != 0 || result.info.Size() > 8<<20 {
		return result, ErrBinding
	}
	result.raw, err = io.ReadAll(io.LimitReader(f, 8<<20+1))
	if err != nil || int64(len(result.raw)) != result.info.Size() {
		return result, errors.Join(ErrBinding, err)
	}
	return result, checkReleaseFile(result)
}

func checkReleaseFile(f *releaseFile) error {
	info, err := f.file.Stat()
	named, ne := os.Lstat(f.path)
	if err != nil || ne != nil || !sameReleaseFile(f.info, info) || !sameReleaseFile(f.info, named) {
		return errors.Join(ErrBinding, err, ne)
	}
	if _, err = f.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(f.file, int64(len(f.raw))+1))
	equal := bytes.Equal(raw, f.raw)
	clear(raw)
	if err != nil || !equal {
		return errors.Join(ErrBinding, err)
	}
	info, err = f.file.Stat()
	named, ne = os.Lstat(f.path)
	if err != nil || ne != nil || !sameReleaseFile(f.info, info) || !sameReleaseFile(f.info, named) {
		return errors.Join(ErrBinding, err, ne)
	}
	return nil
}

func sameReleaseFile(before, after os.FileInfo) bool {
	if before == nil || after == nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() {
		return false
	}
	a, aok := before.Sys().(*syscall.Stat_t)
	b, bok := after.Sys().(*syscall.Stat_t)
	return aok && bok && a.Uid == b.Uid && a.Gid == b.Gid && a.Nlink == b.Nlink && a.Ctim == b.Ctim && a.Mtim == b.Mtim
}

// ObserveUbuntu2404 independently opens and rejoins the original platform file.
func ObserveUbuntu2404(ctx context.Context) (returned error) {
	if ctx == nil {
		return ErrBinding
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := readReleaseFile("/usr/lib/os-release")
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, f.file.Close(), ctx.Err()); clear(f.raw) }()
	if !Ubuntu2404(f.raw) {
		return ErrBinding
	}

	return checkReleaseFile(f)
}

// MapBinding preserves physical errors while using the role's existing refusal.
func MapBinding(err, binding error) error {
	if err == nil {
		return nil
	}
	if err == ErrBinding {
		return binding
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		mapped := make([]error, len(causes))
		for i, cause := range causes {
			mapped[i] = MapBinding(cause, binding)
		}
		return errors.Join(mapped...)
	}
	return err
}
