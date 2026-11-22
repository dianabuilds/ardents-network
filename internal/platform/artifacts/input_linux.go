package artifacts

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

// ErrInput reports an unavailable or invalid original artifact input.
var ErrInput = errors.New("original source preparation input unavailable")

// File retains the original descriptor and metadata of an input or directory.
type File struct {
	file   *os.File
	info   os.FileInfo
	parent *File
	name   string
	raw    []byte
}

// Inputs retains original descriptors for one finite offline operation. The
// caller serializes its use and joins any borrowers before closing it.
type Inputs struct {
	ctx    context.Context
	files  []*File
	failed error
}

func NewInputs(ctx context.Context) *Inputs {
	return &Inputs{ctx: ctx}
}

// Last returns the original input from the most recent successful read.
func (in *Inputs) Last() *File {
	return in.files[len(in.files)-1]
}

func (f *File) Info() os.FileInfo { return f.info }

// Descriptor borrows the original descriptor until its owning Inputs closes.
func (f *File) Descriptor() *os.File { return f.file }

func AbsolutePath(path string) bool {
	return path != "/" && len(path) > 1 && len(path) <= 4096 && filepath.IsAbs(path) && filepath.Clean(path) == path && !strings.ContainsRune(path, 0)
}

func OpenChild(parent *File, name string, directory bool) (*os.File, error) {
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

func (in *Inputs) Retain(file *os.File, parent *File, name string) (*File, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok || s.Uid != 0 && s.Uid != uint32(os.Geteuid()) || !info.IsDir() && (!info.Mode().IsRegular() || s.Nlink != 1 || info.Mode().Perm()&0022 != 0) {
		return nil, errors.Join(ErrInput, file.Close())
	}
	original := &File{file: file, info: info, parent: parent, name: name}
	in.files = append(in.files, original)
	return original, nil
}

func (in *Inputs) Directory(path string) (*File, error) {
	if len(strings.Split(path, "/")) > 128 {
		return nil, ErrInput
	}
	return in.openDirectory(path)
}

func (in *Inputs) openDirectory(path string) (*File, error) {
	if path != "/" && !AbsolutePath(path) {
		return nil, ErrInput
	}
	file, err := os.Open("/")
	if err != nil {
		return nil, err
	}
	parent, err := in.Retain(file, nil, "")
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
		file, err := OpenChild(parent, name, true)
		if err != nil {
			return nil, err
		}
		parent, err = in.Retain(file, parent, name)
		if err != nil {
			return nil, err
		}
	}
	return parent, nil
}

func (in *Inputs) Read(path string, maximum int64) ([]byte, error) {
	if !AbsolutePath(path) || in.ctx == nil || in.ctx.Err() != nil {
		return nil, ErrInput
	}
	parent, err := in.Directory(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	file, err := OpenChild(parent, filepath.Base(path), false)
	if err != nil {
		return nil, err
	}
	original, err := in.Retain(file, parent, filepath.Base(path))
	if err != nil {
		return nil, err
	}
	if !original.info.Mode().IsRegular() || original.info.Size() < 1 || original.info.Size() > maximum {
		return nil, ErrInput
	}
	buffer := make([]byte, int(original.info.Size())+1)
	count, readErr := file.ReadAt(buffer, 0)
	original.raw = buffer[:count]
	if readErr != nil && readErr != io.EOF || int64(count) != original.info.Size() {
		return nil, errors.Join(ErrInput, readErr)
	}
	return original.raw, in.Check()
}

func (in *Inputs) Check() error {
	if in.failed != nil {
		return in.failed
	}
	in.failed = in.observe()
	return in.failed
}

func (in *Inputs) observe() error {
	if in.ctx == nil {
		return ErrInput
	}
	return in.observeRead(func(file *os.File, raw []byte) (int, error) { return file.ReadAt(raw, 0) })
}

func (in *Inputs) observeRead(read func(*os.File, []byte) (int, error)) error {
	if err := in.ctx.Err(); err != nil {
		return err
	}
	for _, original := range in.files {
		current, err := original.file.Stat()
		if err != nil || !os.SameFile(original.info, current) || current.Mode() != original.info.Mode() {
			return errors.Join(ErrInput, err)
		}
		before, after := original.info.Sys().(*syscall.Stat_t), current.Sys().(*syscall.Stat_t)
		if before.Uid != after.Uid || before.Gid != after.Gid {
			return ErrInput
		}
		if original.parent != nil {
			linked, err := OpenChild(original.parent, original.name, current.IsDir())
			if err != nil {
				return err
			}
			info, err := linked.Stat()
			err = errors.Join(err, linked.Close())
			if err != nil || !os.SameFile(original.info, info) {
				return errors.Join(ErrInput, err)
			}
		}
		if current.IsDir() {
			continue
		}
		if after.Ctim != before.Ctim || after.Nlink != 1 || current.Size() != original.info.Size() {
			return ErrInput
		}
		raw := make([]byte, len(original.raw)+1)
		count, err := read(original.file, raw)
		matches := (err == nil || err == io.EOF) && count == len(original.raw) && bytes.Equal(raw[:count], original.raw)
		clear(raw)
		if !matches {
			return errors.Join(ErrInput, err)
		}
		final, err := original.file.Stat()
		if err != nil {
			return err
		}
		last := final.Sys().(*syscall.Stat_t)
		if !os.SameFile(current, final) || final.Mode() != current.Mode() || final.Size() != current.Size() || last.Ctim != after.Ctim || last.Mtim != after.Mtim || last.Nlink != 1 || last.Uid != after.Uid || last.Gid != after.Gid {
			return ErrInput
		}
	}
	// Later file I/O cannot carry an earlier namespace observation through
	// handoff. Recheck every original link and access after all content reads.
	for _, original := range in.files {
		current, err := original.file.Stat()
		if err != nil || !os.SameFile(original.info, current) || current.Mode() != original.info.Mode() {
			return errors.Join(ErrInput, err)
		}
		before, after := original.info.Sys().(*syscall.Stat_t), current.Sys().(*syscall.Stat_t)
		if before.Uid != after.Uid || before.Gid != after.Gid || !current.IsDir() && (after.Ctim != before.Ctim || after.Mtim != before.Mtim || after.Nlink != 1 || current.Size() != original.info.Size()) {
			return ErrInput
		}
		if original.parent != nil {
			linked, err := OpenChild(original.parent, original.name, current.IsDir())
			if err != nil {
				return err
			}
			info, err := linked.Stat()
			err = errors.Join(err, linked.Close())
			if err != nil || !os.SameFile(original.info, info) || info.Mode() != current.Mode() || info.Sys().(*syscall.Stat_t).Uid != after.Uid || info.Sys().(*syscall.Stat_t).Gid != after.Gid {
				return errors.Join(ErrInput, err)
			}
		}
	}
	return in.ctx.Err()
}

func (in *Inputs) Close() (result error) {
	result = in.Check()
	for i := len(in.files) - 1; i >= 0; i-- {
		result = errors.Join(result, in.files[i].file.Close())
		clear(in.files[i].raw)
	}
	return result
}

// Offline root preparation/signing adds protected ancestors and native public
// file storage to the shared original-descriptor checks. Runtime/client input
// custody keeps its separately admitted caller policy.
func (in *Inputs) CheckProtected() error {
	if os.Geteuid() != 0 {
		return ErrInput
	}
	for _, input := range in.files {
		info, err := input.file.Stat()
		if err != nil || info.Sys().(*syscall.Stat_t).Uid != 0 || info.Mode().Perm()&0022 != 0 {
			return errors.Join(ErrInput, err)
		}
		if info.IsDir() {
			continue
		}
		var fs syscall.Statfs_t
		if err := syscall.Fstatfs(int(input.file.Fd()), &fs); err != nil || fs.Type != 0xef53 {
			return errors.Join(ErrInput, err)
		}
	}
	return in.Check()
}
