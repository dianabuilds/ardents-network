package durablefile

import (
	"errors"
	"os"
)

// InspectRoot creates or validates the owned directory for one store root. A
// missing root is created owner-only when create is true and refused with the
// caller's missing-root prose otherwise; a non-directory or symlink root is
// refused with the caller's not-a-directory prose. It grants no lease or claim.
func InspectRoot(root string, create bool, missingRefusal, notDirRefusal string) error {
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		if !create {
			return errors.New(missingRefusal)
		}
		if err := os.MkdirAll(root, 0o700); err != nil {
			return err
		}
		info, err = os.Lstat(root)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New(notDirRefusal)
	}
	return nil
}
