//go:build windows

package custody

import "golang.org/x/sys/windows"

func durableRename(oldPath, newPath string) error {
	oldPointer, err := windows.UTF16PtrFromString(oldPath)
	if err != nil {
		return err
	}
	newPointer, err := windows.UTF16PtrFromString(newPath)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(oldPointer, newPointer, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

// Windows has no directory fsync. Every custody file is flushed before its
// write-through replacement, while native crash/power-loss evidence remains a
// separate supported-platform qualification gate.
func syncDirectory(string) error { return nil }
