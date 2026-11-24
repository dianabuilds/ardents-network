//go:build linux || darwin

package artifacts

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
)

func walkDirectories(ctx context.Context, path string, retain func(*os.File, *File, string) (*File, error)) (*File, error) {
	file, err := os.Open("/")
	if err != nil {
		return nil, err
	}
	parent, err := retain(file, nil, "")
	if err != nil {
		return nil, err
	}
	for _, name := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if name == "" {
			continue
		}
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		file, err := OpenChild(parent, name, true)
		if err != nil {
			return nil, err
		}
		parent, err = retain(file, parent, name)
		if err != nil {
			return nil, err
		}
	}
	return parent, nil
}

// DirectoryChain retains the original path from / through the selected
// directory. Its descriptors are local to a single caller's lifetime.
type DirectoryChain struct {
	directories []*File
}

// OpenDirectoryChain opens each component without following links. When owner
// is supplied each original directory must belong to root or that UID. The
// caller independently admits each selected file before reading its contents.
func OpenDirectoryChain(ctx context.Context, path string, owner *uint32) (*DirectoryChain, error) {
	if !pathgrammar.Strict(path) {
		return nil, ErrInput
	}
	chain := &DirectoryChain{}
	_, err := walkDirectories(ctx, path, func(file *os.File, parent *File, name string) (*File, error) {
		info, err := file.Stat()
		if err != nil {
			return nil, errors.Join(err, file.Close())
		}
		directory := &File{file: file, info: info, parent: parent, name: name}
		chain.directories = append(chain.directories, directory)
		if owner != nil {
			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok || stat.Uid != 0 && stat.Uid != *owner {
				return nil, ErrInput
			}
		}
		return directory, nil
	})
	if err != nil {
		return nil, errors.Join(err, chain.Close())
	}
	return chain, nil
}

func (d *DirectoryChain) last() *File { return d.directories[len(d.directories)-1] }

// Descriptor borrows the original final directory until Close.
func (d *DirectoryChain) Descriptor() *os.File { return d.last().file }

// OpenFile opens a direct entry relative to the retained final directory.
func (d *DirectoryChain) OpenFile(name string) (*os.File, error) {
	if name == "." || name == ".." || filepath.IsAbs(name) || filepath.Base(name) != name {
		return nil, ErrInput
	}
	return OpenChild(d.last(), name, false)
}

// Check reobserves original directory identities, modes and links.
func (d *DirectoryChain) Check() error {
	for _, original := range d.directories {
		current, err := original.file.Stat()
		if err != nil || !os.SameFile(original.info, current) || !current.IsDir() || current.Mode() != original.info.Mode() {
			return errors.Join(ErrInput, err)
		}
		if original.parent == nil {
			continue
		}
		linked, err := OpenChild(original.parent, original.name, true)
		if err != nil {
			return err
		}
		observed, err := linked.Stat()
		err = errors.Join(err, linked.Close())
		if err != nil || !os.SameFile(original.info, observed) {
			return errors.Join(ErrInput, err)
		}
	}
	return nil
}

// CheckOriginalAccess also requires each original owner and group to remain.
func (d *DirectoryChain) CheckOriginalAccess() error {
	return d.checkAccess(false)
}

// CheckRootAccess also requires root ownership, the original group, and no
// group or other write access.
func (d *DirectoryChain) CheckRootAccess() error {
	return d.checkAccess(true)
}

func (d *DirectoryChain) checkAccess(rootControlled bool) error {
	if err := d.Check(); err != nil {
		return err
	}
	for _, original := range d.directories {
		current, err := original.file.Stat()
		if err != nil {
			return err
		}
		before, originalOK := original.info.Sys().(*syscall.Stat_t)
		after, currentOK := current.Sys().(*syscall.Stat_t)
		if !originalOK || !currentOK || before.Gid != after.Gid || current.Mode() != original.info.Mode() {
			return ErrInput
		}
		if rootControlled {
			if after.Uid != 0 || current.Mode().Perm()&0022 != 0 {
				return ErrInput
			}
		} else if !os.SameFile(original.info, current) || before.Uid != after.Uid {
			return ErrInput
		}
	}
	return nil
}

// Close releases the retained directories in reverse acquisition order.
func (d *DirectoryChain) Close() (err error) {
	for i := len(d.directories) - 1; i >= 0; i-- {
		err = errors.Join(err, d.directories[i].file.Close())
	}
	return err
}
