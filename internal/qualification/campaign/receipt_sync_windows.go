//go:build windows

package campaign

import "golang.org/x/sys/windows"

func syncDirectory(path string) error {
	// Windows has no supported directory fsync equivalent. CreateDirectory
	// itself publishes the reservation; the final rename below is write-through.
	return nil
}

func publishReceipt(pending, final, _ string) error {
	from, err := windows.UTF16PtrFromString(pending)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(final)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_WRITE_THROUGH)
}
