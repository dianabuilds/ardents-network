package credential

import (
	"fmt"
	"io"
	"os"
)

const issuerRootLockName = ".ardents-local-roles-lock"

func writeIssuerExclusive(path string, raw []byte) error {
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

func readIssuerFile(path string, maximum int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if int64(len(raw)) > maximum {
		_ = file.Close()
		return nil, fmt.Errorf("bounded transit grant issuer file exceeds %d bytes", maximum)
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return raw, nil
}
