//go:build windows

package release

import (
	"golang.org/x/sys/windows"
)

func durableRename(oldPath, newPath string) error {
	oldPointer, err := windows.UTF16PtrFromString(oldPath)
	if err != nil {
		return err
	}
	newPointer, err := windows.UTF16PtrFromString(newPath)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(
		oldPointer,
		newPointer,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	)
}

// Windows has no directory-fsync operation. Publication uses
// MOVEFILE_WRITE_THROUGH above, and every file is flushed before the rename.
func syncDirectory(string) error { return nil }
