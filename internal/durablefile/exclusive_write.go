package durablefile

import "os"

// WriteExclusive creates one new owner-only file and durably writes its exact
// bytes, refusing any pre-existing target. A write or sync failure is returned
// ahead of the close result; the file is always closed.
func WriteExclusive(path string, raw []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
