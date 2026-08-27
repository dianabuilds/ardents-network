package custody

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func checkedRoot(path string) (string, error) {
	if path == "" {
		return "", ErrInvalid
	}
	root, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("release custody root: %w", err)
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", ErrInvalid
	}
	return root, nil
}

func seedPath(root string) string { return filepath.Join(root, "release-seeds.json") }

func requireAbsent(path string) error {
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err == nil {
		return ErrExists
	}
	return fmt.Errorf("inspect release custody seed record: %w", err)
}

func writeNew(path string, data []byte) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".release-seeds-")
	if err != nil {
		return fmt.Errorf("create release custody staging file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("protect release custody staging file: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write release custody staging file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("flush release custody staging file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close release custody staging file: %w", err)
	}
	if err := requireAbsent(path); err != nil {
		return err
	}
	if err := os.Link(temporaryPath, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrExists
		}
		return fmt.Errorf("publish release custody seed record: %w", err)
	}
	published, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("reopen release custody seed record: %w", err)
	}
	defer published.Close()
	publishedBytes, err := io.ReadAll(published)
	if err != nil {
		return fmt.Errorf("read release custody seed record: %w", err)
	}
	if !bytes.Equal(publishedBytes, data) {
		return ErrInvalid
	}
	return nil
}
