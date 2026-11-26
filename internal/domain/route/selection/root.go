package selection

import (
	"os"

	"github.com/dianabuilds/ardents-network/internal/durablefile"
)

// rootLockName is the exclusive-lease file shared by every claimed Entry
// root. The retired Invite root used the same lock name and its own
// `.ardents-entry-state-v1` marker (ADR-0106 deleted both); the closed
// Entry set root refuses any foreign population through its own marker and
// allowed-name inspection.
const rootLockName = ".ardents-entry-state-lock"

// inspectRoot creates or validates the owned directory for one Entry root.
func inspectRoot(root string) error {
	return durablefile.InspectRoot(root, true, "", "entry state root is not an owned directory")
}

// validateRootPermissions enforces the owner-only access floor for one
// claimed Entry root through the platform owner check.
func validateRootPermissions(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	return validateOwnerOnlyRoot(root, info)
}

// writeExclusive creates one new owner-only file and durably writes its
// exact bytes, refusing any pre-existing target.
func writeExclusive(path string, raw []byte) error {
	return durablefile.WriteExclusive(path, raw)
}
