//go:build !windows

package durablefile

import "os"

// SyncDirectory flushes one directory's entries to stable storage. A sync
// failure is returned ahead of the close result; the directory is always closed.
func SyncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	err = directory.Sync()
	closeErr := directory.Close()
	if err != nil {
		return err
	}
	return closeErr
}
