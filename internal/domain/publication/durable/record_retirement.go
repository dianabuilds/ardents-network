package durable

import (
	"fmt"
	"os"
	"path/filepath"
)

// Only the record independently reconciled by Open may be retired. A visible
// unlink is not completion: pointer retirement and record/directory removal
// retain their own required parent barriers before the next floor is written.
func (root *Root) retireRecord() error {
	if !root.current {
		return nil
	}
	if err := os.Remove(filepath.Join(root.path, "current")); err != nil {
		return err
	}
	if err := root.flush(root.path); err != nil {
		return err
	}
	path := filepath.Join(root.path, "generations", fmt.Sprintf("%016x", root.floor))
	if err := os.Remove(filepath.Join(path, "publication.bin")); err != nil {
		return err
	}
	if err := root.flush(path); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	if err := root.flush(filepath.Dir(path)); err != nil {
		return err
	}
	root.current = false
	return nil
}
