package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

var errSourceControlInput = errors.New("original source preparation input unavailable")

type sourceControlFile struct {
	file   *os.File
	info   os.FileInfo
	parent *sourceControlFile
	name   string
	raw    []byte
}

// The finite caller serializes these original descriptors. State is closed
// before their release; no detached observer, borrowed runtime root or key is
// retained. Directory contents may change through State's genuine operations.
type sourceControlInputs struct {
	ctx    context.Context
	files  []*sourceControlFile
	failed error
}

func sourceControlPath(path string) bool {
	return path != "/" && len(path) > 1 && len(path) <= 4096 && filepath.IsAbs(path) && filepath.Clean(path) == path && !strings.ContainsRune(path, 0)
}

func sourceControlOpen(parent *sourceControlFile, name string, directory bool) (*os.File, error) {
	flags := syscall.O_RDONLY | syscall.O_CLOEXEC | syscall.O_NOFOLLOW | syscall.O_NONBLOCK
	if directory {
		flags |= syscall.O_DIRECTORY
	}
	fd, err := syscall.Openat(int(parent.file.Fd()), name, flags, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}

func (in *sourceControlInputs) retain(file *os.File, parent *sourceControlFile, name string) (*sourceControlFile, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok || s.Uid != 0 && s.Uid != uint32(os.Geteuid()) || !info.IsDir() && (!info.Mode().IsRegular() || s.Nlink != 1 || info.Mode().Perm()&0022 != 0) {
		return nil, errors.Join(errSourceControlInput, file.Close())
	}
	original := &sourceControlFile{file: file, info: info, parent: parent, name: name}
	in.files = append(in.files, original)
	return original, nil
}

func (in *sourceControlInputs) directory(path string) (*sourceControlFile, error) {
	if path != "/" && !sourceControlPath(path) || len(strings.Split(path, "/")) > 128 {
		return nil, errSourceControlInput
	}
	file, err := os.Open("/")
	if err != nil {
		return nil, err
	}
	parent, err := in.retain(file, nil, "")
	if err != nil {
		return nil, err
	}
	for _, name := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if name == "" {
			continue
		}
		if err := in.ctx.Err(); err != nil {
			return nil, err
		}
		file, err := sourceControlOpen(parent, name, true)
		if err != nil {
			return nil, err
		}
		parent, err = in.retain(file, parent, name)
		if err != nil {
			return nil, err
		}
	}
	return parent, nil
}

func (in *sourceControlInputs) read(path string, maximum int64) ([]byte, error) {
	if !sourceControlPath(path) || in.ctx == nil || in.ctx.Err() != nil {
		return nil, errSourceControlInput
	}
	parent, err := in.directory(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	file, err := sourceControlOpen(parent, filepath.Base(path), false)
	if err != nil {
		return nil, err
	}
	original, err := in.retain(file, parent, filepath.Base(path))
	if err != nil {
		return nil, err
	}
	if !original.info.Mode().IsRegular() || original.info.Size() < 1 || original.info.Size() > maximum {
		return nil, errSourceControlInput
	}
	buffer := make([]byte, int(original.info.Size())+1)
	count, readErr := file.ReadAt(buffer, 0)
	original.raw = buffer[:count]
	if readErr != nil && readErr != io.EOF || int64(count) != original.info.Size() {
		return nil, errors.Join(errSourceControlInput, readErr)
	}
	return original.raw, in.check()
}

func (in *sourceControlInputs) check() error {
	if in.failed != nil {
		return in.failed
	}
	in.failed = in.observe()
	return in.failed
}

func (in *sourceControlInputs) observe() error {
	return in.observeRead(func(file *os.File, raw []byte) (int, error) { return file.ReadAt(raw, 0) })
}

func (in *sourceControlInputs) observeRead(read func(*os.File, []byte) (int, error)) error {
	if err := in.ctx.Err(); err != nil {
		return err
	}
	for _, original := range in.files {
		current, err := original.file.Stat()
		if err != nil || !os.SameFile(original.info, current) || current.Mode() != original.info.Mode() {
			return errors.Join(errSourceControlInput, err)
		}
		before, after := original.info.Sys().(*syscall.Stat_t), current.Sys().(*syscall.Stat_t)
		if before.Uid != after.Uid || before.Gid != after.Gid {
			return errSourceControlInput
		}
		if original.parent != nil {
			linked, err := sourceControlOpen(original.parent, original.name, current.IsDir())
			if err != nil {
				return err
			}
			info, err := linked.Stat()
			err = errors.Join(err, linked.Close())
			if err != nil || !os.SameFile(original.info, info) {
				return errors.Join(errSourceControlInput, err)
			}
		}
		if current.IsDir() {
			continue
		}
		if after.Ctim != before.Ctim || after.Nlink != 1 || current.Size() != original.info.Size() {
			return errSourceControlInput
		}
		raw := make([]byte, len(original.raw)+1)
		count, err := read(original.file, raw)
		matches := (err == nil || err == io.EOF) && count == len(original.raw) && bytes.Equal(raw[:count], original.raw)
		clear(raw)
		if !matches {
			return errors.Join(errSourceControlInput, err)
		}
		final, err := original.file.Stat()
		if err != nil {
			return err
		}
		last := final.Sys().(*syscall.Stat_t)
		if !os.SameFile(current, final) || final.Mode() != current.Mode() || final.Size() != current.Size() || last.Ctim != after.Ctim || last.Mtim != after.Mtim || last.Nlink != 1 || last.Uid != after.Uid || last.Gid != after.Gid {
			return errSourceControlInput
		}
	}
	// Later file I/O cannot carry an earlier namespace observation through
	// handoff. Recheck every original link and access after all content reads.
	for _, original := range in.files {
		current, err := original.file.Stat()
		if err != nil || !os.SameFile(original.info, current) || current.Mode() != original.info.Mode() {
			return errors.Join(errSourceControlInput, err)
		}
		before, after := original.info.Sys().(*syscall.Stat_t), current.Sys().(*syscall.Stat_t)
		if before.Uid != after.Uid || before.Gid != after.Gid || !current.IsDir() && (after.Ctim != before.Ctim || after.Mtim != before.Mtim || after.Nlink != 1 || current.Size() != original.info.Size()) {
			return errSourceControlInput
		}
		if original.parent != nil {
			linked, err := sourceControlOpen(original.parent, original.name, current.IsDir())
			if err != nil {
				return err
			}
			info, err := linked.Stat()
			err = errors.Join(err, linked.Close())
			if err != nil || !os.SameFile(original.info, info) || info.Mode() != current.Mode() || info.Sys().(*syscall.Stat_t).Uid != after.Uid || info.Sys().(*syscall.Stat_t).Gid != after.Gid {
				return errors.Join(errSourceControlInput, err)
			}
		}
	}
	return in.ctx.Err()
}

func (in *sourceControlInputs) close() (result error) {
	result = in.check()
	for i := len(in.files) - 1; i >= 0; i-- {
		result = errors.Join(result, in.files[i].file.Close())
		clear(in.files[i].raw)
	}
	return result
}

// Offline root preparation/signing adds protected ancestors and native public
// file storage to the shared original-descriptor checks. Runtime/client input
// custody keeps its separately admitted caller policy.
func (in *sourceControlInputs) checkProtected() error {
	if os.Geteuid() != 0 {
		return errSourceControlInput
	}
	for _, input := range in.files {
		info, err := input.file.Stat()
		if err != nil || info.Sys().(*syscall.Stat_t).Uid != 0 || info.Mode().Perm()&0022 != 0 {
			return errors.Join(errSourceControlInput, err)
		}
		if info.IsDir() {
			continue
		}
		var fs syscall.Statfs_t
		if err := syscall.Fstatfs(int(input.file.Fd()), &fs); err != nil || fs.Type != 0xef53 {
			return errors.Join(errSourceControlInput, err)
		}
	}
	return in.check()
}
