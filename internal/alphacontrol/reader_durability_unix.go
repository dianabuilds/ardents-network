//go:build !windows

package alphacontrol

import "os"

func durableReaderRename(oldPath, newPath string) error { return os.Rename(oldPath, newPath) }

func syncReaderDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
