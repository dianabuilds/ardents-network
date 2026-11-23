//go:build windows

package selection

import (
	"os"

	"github.com/dianabuilds/ardents-network/internal/durablefile"
)

// validateOwnerOnlyRoot enforces the owner-only permission floor on Windows
// through the shared protected owner-only DACL mechanism (#793). Go
// synthesizes Windows mode bits from FILE_ATTRIBUTE_READONLY rather than the
// DACL, so a permission-bit check can never observe the floor; enforcement by
// mutation is the established Windows pattern (duty). The Unix sibling
// additionally binds the root to the endpoint user's UID (audit SW-9); the
// DACL grant binds the root to the current process owner SID.
func validateOwnerOnlyRoot(root string, _ os.FileInfo) error {
	return durablefile.EnforceOwnerOnlyRoot(root, durablefile.OwnerOnlyRootRefusal{
		OwnerSID: "read entry state owner SID",
		RootDACL: "entry state root owner-only DACL could not be enforced",
		ReadRoot: "read entry state root for DACL enforcement",
		FileDACL: "entry state file owner-only DACL could not be enforced",
	})
}
