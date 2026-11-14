package descriptor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Native unit-search/context custody is independent of delivery-file custody.
// Only the reload owner may replace the three actual generator observations.
type nativeDirectories struct {
	ctx    context.Context
	dirs   []*inputDirectory
	byPath map[string]*inputDirectory
}
type inputDirectory struct {
	path, name string
	file       *os.File
	info       os.FileInfo
	parent     *inputDirectory
}

func (p *nativeDirectories) close() error {
	if p == nil {
		return nil
	}
	var result error
	for i := len(p.dirs) - 1; i >= 0; i-- {
		result = errors.Join(result, p.dirs[i].file.Close())
	}
	p.dirs = nil
	p.byPath = nil
	return errors.Join(result, p.ctx.Err())
}

func (p *nativeDirectories) parents(path string) (*inputDirectory, error) {
	directory := filepath.Dir(path)
	var parent *inputDirectory
	parts := append([]string{""}, strings.Split(strings.TrimPrefix(directory, "/"), "/")...)
	current := "/"
	for i, name := range parts {
		if i > 0 && name == "" {
			continue
		}
		if i > 0 {
			current = filepath.Join(current, name)
		}
		if retained := p.byPath[current]; retained != nil {
			parent = retained
			continue
		}
		var file *os.File
		var err error
		if parent == nil {
			file, err = os.OpenFile("/", os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
		} else {
			file, err = openInputAt(parent, name, true)
		}
		if err != nil {
			return nil, err
		}
		original := &inputDirectory{path: current, name: name, file: file, parent: parent}
		p.dirs = append(p.dirs, original)
		original.info, err = file.Stat()
		if err != nil || !trustedInputDirectory(original.info) {
			return nil, errors.Join(ErrBinding, err)
		}
		p.byPath[current] = original
		parent = original
	}
	return parent, nil
}

func openInputAt(parent *inputDirectory, name string, directory bool) (*os.File, error) {
	flags := syscall.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK | syscall.O_CLOEXEC
	if directory {
		flags |= syscall.O_DIRECTORY
	}
	fd, err := syscall.Openat(int(parent.file.Fd()), name, flags, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}

func (p *nativeDirectories) links() error {
	for _, original := range p.dirs {
		current, err := original.file.Stat()
		if err != nil || !trustedInputDirectory(current) || !os.SameFile(original.info, current) || original.info.Mode() != current.Mode() {
			return errors.Join(ErrBinding, err)
		}
		if original.parent != nil {
			linked, err := openInputAt(original.parent, original.name, true)
			if err != nil {
				return err
			}
			named, statErr := linked.Stat()
			if err := errors.Join(statErr, linked.Close()); err != nil || !trustedInputDirectory(named) || !os.SameFile(original.info, named) || original.info.Mode() != named.Mode() {
				return errors.Join(ErrBinding, err)
			}
		}
	}
	return nil
}
