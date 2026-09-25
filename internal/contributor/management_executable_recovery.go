package contributor

import (
	"errors"
	"os"
	"path/filepath"
)

// restoreManagementExecutableFromCurrent repairs the management copy only
// after the current generation has been authenticated by recovery.
func (profile *Profile) restoreManagementExecutableFromCurrent() error {
	raw, err := readRegular(filepath.Join(profile.paths.programCurrent, "ardents-node"), 128<<20)
	if err != nil {
		return err
	}
	return writeFileAtomic(profile.paths.programManagement, raw, 0o755)
}

func writeFileAtomic(path string, raw []byte, mode os.FileMode) error {
	temporary := path + ".new"
	if err := os.Remove(temporary); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(raw)
	if writeErr == nil && written != len(raw) {
		writeErr = errors.New("short Contributor file write")
	}
	if err := errors.Join(writeErr, file.Sync(), file.Close()); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
