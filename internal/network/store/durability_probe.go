package store

import (
	"errors"
	"fmt"
	"os"
)

func verifyRootWritable(root string) error {
	probe, err := os.CreateTemp(root, ".current-durability-")
	if err != nil {
		return fmt.Errorf("create state durability probe: %w", err)
	}
	path := probe.Name()
	block := make([]byte, 4096)
	_, writeErr := probe.Write(block)
	if writeErr == nil {
		writeErr = probe.Sync()
	}
	closeErr := probe.Close()
	removeErr := os.Remove(path)
	if err := errors.Join(writeErr, closeErr, removeErr); err != nil {
		return fmt.Errorf("complete state durability probe: %w", err)
	}
	if err := syncDirectory(root); err != nil {
		return fmt.Errorf("sync state durability probe: %w", err)
	}
	return nil
}
