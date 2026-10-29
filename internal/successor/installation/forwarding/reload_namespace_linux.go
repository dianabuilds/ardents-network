package forwarding

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
)

// A successful owned Reload may recreate exactly the three systemd generator
// outputs. Every other original ancestor and absence remains pinned. This
// physical observer cannot reload, select policy or grant a recovery right.
func (n *startNamespace) reobserveAfterReload() (returned error) {
	if n == nil || n.closed || n.parents == nil || n.terminal != nil {
		return ErrBinding
	}
	defer func() {
		if returned != nil {
			n.terminal = errors.Join(n.terminal, returned)
		}
	}()
	next, err := openStartNamespace(n.parents.ctx, n.paths)
	if next != nil {
		defer func() { returned = errors.Join(returned, next.close()) }()
	}
	if err != nil {
		return err
	}
	generated := func(path string) bool {
		return path == "/run/systemd/generator" || path == "/run/systemd/generator.early" || path == "/run/systemd/generator.late"
	}
	for _, original := range n.parents.dirs {
		if generated(original.path) {
			continue
		}
		current := next.parents.byPath[original.path]
		if current == nil || !os.SameFile(original.info, current.info) || original.info.Mode() != current.info.Mode() || !trustedInputDirectory(current.info) {
			return ErrBinding
		}
	}
	for _, path := range n.absent {
		if !generated(path) && !slices.Contains(next.absent, path) {
			return ErrBinding
		}
	}
	for _, path := range []string{"/run/systemd/generator", "/run/systemd/generator.early", "/run/systemd/generator.late"} {
		if original := n.parents.byPath[path]; original != nil {
			if err := original.file.Close(); err != nil {
				return err
			}
			delete(n.parents.byPath, path)
			n.parents.dirs = slices.DeleteFunc(n.parents.dirs, func(directory *inputDirectory) bool { return directory == original })
		}
		n.absent = slices.DeleteFunc(n.absent, func(absent string) bool { return absent == path })
		if slices.Contains(next.absent, path) {
			n.absent = append(n.absent, path)
		}
		if current := next.parents.byPath[path]; current != nil {
			parent := n.parents.byPath[filepath.Dir(path)]
			if parent == nil {
				return ErrBinding
			}
			// The transferred original descriptor must refer to our retained
			// parent, not the fresh observer's soon-to-close parent descriptor.
			current.parent = parent
			n.parents.byPath[path] = current
			n.parents.dirs = append(n.parents.dirs, current)
			delete(next.parents.byPath, path)
			next.parents.dirs = slices.DeleteFunc(next.parents.dirs, func(directory *inputDirectory) bool { return directory == current })
		}
	}
	return n.check()
}
