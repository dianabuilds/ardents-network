//go:build windows

package selection

import (
	"os"

	"github.com/dianabuilds/ardents-network/internal/durablefile"
)

// validateOwnerOnlyRoot checks the original Windows root and entries without
// changing their owners or DACLs.
func validateOwnerOnlyRoot(root string, _ os.FileInfo) error {
	return durablefile.EnforceOwnerOnlyRoot(root, durablefile.OwnerOnlyRootRefusal{
		OwnerSID: "read entry state owner SID",
		RootDACL: "entry state root owner-only DACL could not be enforced",
		ReadRoot: "read entry state root for DACL enforcement",
		FileDACL: "entry state file owner-only DACL could not be enforced",
	})
}
