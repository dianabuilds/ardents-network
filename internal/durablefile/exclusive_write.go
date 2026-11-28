package durablefile

// WriteExclusive creates one new owner-only file and durably writes its exact
// bytes, refusing any pre-existing target. A write or sync failure is returned
// ahead of the close result; the file is always closed.
func WriteExclusive(path string, raw []byte) error {
	file, err := createOwnerOnlyFile(path)
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
