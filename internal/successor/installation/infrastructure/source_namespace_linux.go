package infrastructure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

// sourceNamespace retains original unit-search ancestors and required absence.
// It grants no reload, manager, Start or runtime authority. The owning Source
// lifetime serializes checks and closes these independent descriptors after use.
type sourceNamespace struct {
	ctx         context.Context
	directories map[string]*os.File
	identities  map[string]os.FileInfo
	absent      map[string]bool
	terminal    error
	closed      bool
	paths       []string
}

func openSourceNamespace(ctx context.Context, paths []string) (result *sourceNamespace, err error) {
	if ctx == nil || len(paths) == 0 || len(paths) > 32 {
		return nil, ErrInput
	}
	n := &sourceNamespace{ctx: ctx, directories: map[string]*os.File{}, identities: map[string]os.FileInfo{}, absent: map[string]bool{}, paths: append([]string(nil), paths...)}
	defer func() {
		if err != nil {
			n.terminal = err
			err = errors.Join(err, n.close())
			result = nil
		}
	}()
	for _, directory := range paths {
		if !sourcePath(directory) {
			return nil, ErrBinding
		}
		for _, leaf := range []string{"ardents-source.service", "ardents-source.service.d", "ardents-source.socket", "ardents-source.socket.d", "ardents-.service.d", "service.d", "socket.d"} {
			path := filepath.Join(directory, leaf)
			// The selected file itself has separate original byte/inode custody.
			if path != "/etc/systemd/system/ardents-source.service" {
				if _, e := os.Lstat(path); !errors.Is(e, os.ErrNotExist) {
					return nil, errors.Join(ErrBinding, e)
				}
				n.absent[path] = true
			}
		}
		for path := directory; ; path = filepath.Dir(path) {
			if err = ctx.Err(); err != nil {
				return nil, err
			}
			if _, exists := n.identities[path]; !exists {
				info, e := os.Lstat(path)
				if errors.Is(e, os.ErrNotExist) {
					n.absent[path] = true
				} else {
					if e != nil || !trustedDirectory(info) {
						return nil, errors.Join(ErrBinding, e)
					}
					file, opened, e := openSourceDirectory(path, info.Mode().Perm(), 0, 0)
					if file != nil {
						n.directories[path] = file
					}
					if e != nil {
						return nil, e
					}
					n.identities[path] = opened
				}
			}
			if path == "/" {
				break
			}
		}
	}
	return n, n.check()
}

func (n *sourceNamespace) check() (err error) {
	if n == nil || n.ctx == nil || n.closed {
		return ErrInput
	}
	if n.terminal != nil {
		return n.terminal
	}
	defer func() {
		err = errors.Join(err, n.ctx.Err())
		if err != nil {
			n.terminal = err
		}
	}()
	if err = n.ctx.Err(); err != nil {
		return err
	}
	for path, info := range n.identities {
		if err = observeSourceDirectory(path, n.directories[path], info); err != nil {
			return err
		}
	}
	for path := range n.absent {
		if _, e := os.Lstat(path); !errors.Is(e, os.ErrNotExist) {
			return errors.Join(ErrBinding, e)
		}
	}
	// An absence read can yield while an earlier original directory is replaced.
	for path, info := range n.identities {
		if err = observeSourceDirectory(path, n.directories[path], info); err != nil {
			return err
		}
	}
	return n.ctx.Err()
}

func (n *sourceNamespace) close() error {
	if n == nil {
		return nil
	}
	if !n.closed {
		n.closed = true
		for _, file := range n.directories {
			n.terminal = errors.Join(n.terminal, file.Close())
		}
		n.terminal = errors.Join(n.terminal, n.ctx.Err())
	}
	return n.terminal
}
