package infrastructure

import (
	"errors"
	"os"
)

// A successful owned daemon reload recreates only these three output
// directories. Preserve every other original namespace identity/absence;
// acquire independent exact new generator pins and join the replaced observers.
// This method neither reloads the manager nor grants native admission.
func (n *sourceNamespace) reobserveAfterReload() (err error) {
	if n == nil || n.closed || n.ctx == nil || n.terminal != nil {
		return ErrBinding
	}
	defer func() {
		if err != nil {
			n.terminal = errors.Join(n.terminal, err)
		}
	}()
	next, err := openSourceNamespace(n.ctx, n.paths)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, next.close()) }()
	generated := func(path string) bool {
		return path == "/run/systemd/generator" || path == "/run/systemd/generator.early" || path == "/run/systemd/generator.late"
	}
	for path, original := range n.identities {
		if generated(path) {
			continue
		}
		current := next.identities[path]
		if current == nil || !os.SameFile(original, current) || original.Mode() != current.Mode() {
			return ErrBinding
		}
	}
	for path := range n.absent {
		if !generated(path) && !next.absent[path] {
			return ErrBinding
		}
	}
	for _, path := range []string{"/run/systemd/generator", "/run/systemd/generator.early", "/run/systemd/generator.late"} {
		if original := n.directories[path]; original != nil {
			if err := original.Close(); err != nil {
				return err
			}
			delete(n.directories, path)
			delete(n.identities, path)
		}
		delete(n.absent, path)
		if next.absent[path] {
			n.absent[path] = true
		}
		if next.directories[path] != nil {
			n.directories[path], n.identities[path] = next.directories[path], next.identities[path]
			delete(next.directories, path)
			delete(next.identities, path)
		}
	}
	return n.check()
}
