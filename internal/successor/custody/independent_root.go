package custody

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
)

var independentRootRecord = []byte("ardents-custody-independent-root-v1\n")

// admitIndependentRoot records fresh development-root birth. Existing records
// or floors cannot obtain signing provenance through a renamed owner. Generic
// encrypted evidence operations do not create this marker or adopt authority.
func (vault *Vault) admitIndependentRoot() error {
	path := filepath.Join(vault.root, "independent-root")
	raw, err := readSmallFile(path)
	if err == nil {
		if !bytes.Equal(raw, independentRootRecord) {
			return ErrInvalid
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	empty, err := vault.isEmpty()
	if err != nil {
		return err
	}
	if !empty {
		return ErrInvalid
	}
	if _, err := os.Lstat(vault.floors); !errors.Is(err, os.ErrNotExist) {
		return ErrInvalid
	}
	if err := vault.check(); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(independentRootRecord)
	syncErr := file.Sync()
	closeErr := file.Close()
	return errors.Join(writeErr, syncErr, closeErr, syncDirectory(vault.root))
}

func (vault *Vault) requireIndependentRoot() error {
	raw, err := readSmallFile(filepath.Join(vault.root, "independent-root"))
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, independentRootRecord) {
		return ErrInvalid
	}
	return nil
}
