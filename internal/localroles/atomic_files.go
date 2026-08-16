package localroles

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	return syncDirectory(root)
}

func replaceFile(root, prefix, name, content string) error {
	temporary, err := os.CreateTemp(root, "."+prefix+"-")
	if err != nil {
		return err
	}
	path := temporary.Name()
	defer func() { _ = os.Remove(path) }()
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.WriteString(content)
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
	if err := os.Rename(path, filepath.Join(root, name)); err != nil {
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

func readBounded(path string, maximum int64) ([]byte, error) {
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
		return nil, fmt.Errorf("bounded local role file exceeds %d bytes", maximum)
	}
	closeErr := file.Close()
	if closeErr != nil {
		return nil, closeErr
	}
	return raw, nil
}
