package entry

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func writeGeneration(root, final string, raw []byte) error {
	temporary, err := os.CreateTemp(root, ".stage-")
	if err != nil {
		return err
	}
	path := temporary.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(path)
		}
	}()
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
		if existing, readErr := readBounded(final, maximumStateBytes); readErr != nil || !bytes.Equal(existing, raw) {
			return err
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	committed = true
	return syncDirectory(root)
}

func replaceCurrent(root, name string) error {
	temporary, err := os.CreateTemp(root, ".current-")
	if err != nil {
		return err
	}
	path := temporary.Name()
	defer func() { _ = os.Remove(path) }()
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.WriteString(name + "\n")
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
	if err := os.Rename(path, filepath.Join(root, "current")); err != nil {
		return err
	}
	return syncDirectory(root)
}

func cleanupGenerations(root, current, previous string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "state-") {
			continue
		}
		name := strings.TrimPrefix(entry.Name(), "state-")
		if name != current && name != previous {
			if err := os.Remove(filepath.Join(root, entry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return syncDirectory(root)
}
