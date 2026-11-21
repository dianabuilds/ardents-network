//go:build windows

package selection

import (
	"errors"
	"os"
)

// validateOwnerOnlyRoot enforces the owner-only permission floor on Windows
// through the access bits Go derives from the file DACL. The Unix sibling
// additionally binds the root to the endpoint user's UID (audit SW-9); the
// equivalent Windows owner-SID verification belongs to the separately selected
// platform-portability work and is not fabricated here.
func validateOwnerOnlyRoot(_ string, info os.FileInfo) error {
	if info.Mode().Perm()&0o077 != 0 {
		return errors.New("entry state root permits group or other access")
	}
	return nil
}
