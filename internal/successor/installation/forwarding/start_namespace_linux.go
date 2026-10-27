package forwarding

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

// startNamespace owns root's fresh unit-search ancestors independently of the
// completed installation and non-root Startup. Its caller serializes its use.
type startNamespace struct {
	parents  *inputs
	absent   []string
	closed   bool
	terminal error
}

func openStartNamespace(ctx context.Context, paths []string) (n *startNamespace, returned error) {
	n = &startNamespace{parents: &inputs{ctx: ctx, byPath: map[string]*inputDirectory{}}}
	if ctx == nil || os.Geteuid() != 0 || len(paths) == 0 || len(paths) > 32 {
		return n, ErrInput
	}
	defer func() {
		if returned != nil {
			n.terminal = returned
		}
	}()
	for _, directory := range paths {
		if !validPath(directory) {
			return n, ErrBinding
		}
		prefix := directory
		for {
			_, err := os.Lstat(prefix)
			if err == nil {
				if _, err := n.parents.parents(filepath.Join(prefix, "observation")); err != nil {
					return n, err
				}
				break
			}
			if !errors.Is(err, os.ErrNotExist) || prefix == "/" {
				return n, errors.Join(ErrBinding, err)
			}
			n.absent = append(n.absent, prefix)
			prefix = filepath.Dir(prefix)
		}
		for _, role := range []string{"forwarding", "issuer", "descriptor", "introduction", "join"} {
			for _, suffix := range []string{".service", ".service.d", ".socket", ".socket.d"} {
				path := filepath.Join(directory, "ardents-"+role+suffix)
				if path != "/etc/systemd/system/ardents-forwarding.service" {
					n.absent = append(n.absent, path)
				}
			}
		}
		for _, leaf := range []string{"ardents-.service.d", "service.d", "socket.d"} {
			n.absent = append(n.absent, filepath.Join(directory, leaf))
		}
	}
	return n, n.check()
}

func (n *startNamespace) check() (returned error) {
	if n == nil || n.parents == nil || n.parents.ctx == nil || n.closed {
		return ErrInput
	}
	if n.terminal != nil {
		return n.terminal
	}
	defer func() {
		if returned != nil {
			n.terminal = returned
		}
	}()
	if err := n.parents.ctx.Err(); err != nil {
		return err
	}
	if err := n.parents.links(); err != nil {
		return err
	}
	for _, path := range n.absent {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
	}
	return errors.Join(n.parents.links(), n.parents.ctx.Err())
}

func (n *startNamespace) close() error {
	if n == nil {
		return nil
	}
	if !n.closed {
		n.closed = true
		if n.parents != nil && n.parents.ctx != nil {
			n.terminal = errors.Join(n.terminal, (Inputs{original: n.parents}).Close())
		}
	}
	return n.terminal
}
