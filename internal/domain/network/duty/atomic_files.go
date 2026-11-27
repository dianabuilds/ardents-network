package duty

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/durablefile"
)

func writeGeneration(root, name string, raw []byte) error {
	final := filepath.Join(root, "state-"+name)
	if existing, err := readBounded(final, maximumStateBytes); err == nil {
		if bytes.Equal(existing, raw) {
			return nil
		}
		return errors.New("immutable local role generation disagrees")
	} else if !os.IsNotExist(err) {
		return err
	}
	temporary, err := os.CreateTemp(root, ".stage-")
	if err != nil {
		return err
	}
	path := temporary.Name()
	defer func() { _ = os.Remove(path) }()
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(raw)
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(path, final); err != nil {
		return err
	}
	return durablefile.SyncDirectory(root)
}

func replaceFile(root, prefix, name, content string) error {
	return durablefile.ReplaceFile(root, "."+prefix+"-", name, []byte(content))
}

func cleanupGenerations(root, current, previous string) error {
	return durablefile.CleanupGenerations(root, current, previous)
}

func readBounded(path string, maximum int64) ([]byte, error) {
	return durablefile.ReadBounded(path, maximum, fmt.Sprintf("bounded local role file exceeds %d bytes", maximum))
}
