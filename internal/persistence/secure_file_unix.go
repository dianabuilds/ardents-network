//go:build !windows

package persistence

import (
	"fmt"
	"os"
)

func validatePrivateFile(_ string, info os.FileInfo) error {
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("private state permissions allow group or other access")
	}
	return nil
}

func protectPrivatePath(_ string, _ bool) error { return nil }

func replacePrivateFile(source, target string) error {
	return os.Rename(source, target)
}
