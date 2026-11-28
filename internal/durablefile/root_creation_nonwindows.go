//go:build !windows

package durablefile

import "os"

func createOwnedRoot(root string) error {
	return os.MkdirAll(root, 0o700)
}
