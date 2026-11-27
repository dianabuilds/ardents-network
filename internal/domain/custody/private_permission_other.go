//go:build !linux

package custody

import "os"

// Mode bits do not establish the owner-only ACL required for this output on
// macOS or Windows. Refuse before the Vault or a staging file is opened.
func checkPrivatePermissionDestination(string) error  { return ErrPrivatePermissionUnavailable }
func checkPrivatePermissionFilesystem(*os.File) error { return ErrPrivatePermissionUnavailable }
func privatePermissionFile(os.FileInfo) bool          { return false }
