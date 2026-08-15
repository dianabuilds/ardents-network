//go:build !windows

package campaign

import "os"

func publishManifest(pending, final, root string) error {
	if err := os.Link(pending, final); err != nil {
		return err
	}
	if err := os.Remove(pending); err != nil {
		return err
	}
	return syncDirectory(root)
}
