package duty

import (
	"errors"
	"os"

	"github.com/dianabuilds/ardents-network/internal/durablefile"
)

func validateRetainedRootPermissions(string) error {
	// No Windows retained participation recovery path is selected.
	return errors.New("retained participation repair is unavailable on Windows")
}

func validateOwnerOnlyRoot(root string, _ os.FileInfo) error {
	return durablefile.EnforceOwnerOnlyRoot(root, durablefile.OwnerOnlyRootRefusal{
		OwnerSID: "read local role owner SID",
		RootDACL: "local role root owner-only DACL could not be enforced",
		ReadRoot: "read local role root for DACL enforcement",
		FileDACL: "local role file owner-only DACL could not be enforced",
	})
}
