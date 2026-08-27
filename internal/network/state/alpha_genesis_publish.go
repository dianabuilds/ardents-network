package state

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func publishAlphaGenesis(ctx context.Context, root, target string, envelope, public []byte, beforeCommit func()) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(root, ".functional-alpha-state-")
	if err != nil {
		return fmt.Errorf("create functional alpha State staging directory: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := os.Chmod(stage, 0o700); err != nil {
		return fmt.Errorf("protect functional alpha State staging directory: %w", err)
	}
	for name, raw := range map[string][]byte{alphaGenesisSeedFile: envelope, alphaGenesisPublicFile: public} {
		if err := writeAlphaGenesisFile(filepath.Join(stage, name), raw); err != nil {
			return err
		}
	}
	for name, expected := range map[string][]byte{alphaGenesisSeedFile: envelope, alphaGenesisPublicFile: public} {
		actual, err := os.ReadFile(filepath.Join(stage, name))
		if err != nil || !bytes.Equal(actual, expected) {
			return ErrAlphaGenesisInvalid
		}
	}
	if beforeCommit != nil {
		beforeCommit()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := os.Lstat(target); err == nil {
		return ErrAlphaGenesisExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("recheck functional alpha State output: %w", err)
	}
	if err := os.Rename(stage, target); err != nil {
		if _, checkErr := os.Lstat(target); checkErr == nil {
			return ErrAlphaGenesisExists
		}
		return fmt.Errorf("publish functional alpha State output: %w", err)
	}
	stage = ""
	return nil
}

func writeAlphaGenesisFile(path string, raw []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create functional alpha State file: %w", err)
	}
	if _, err := file.Write(raw); err != nil {
		file.Close()
		return fmt.Errorf("write functional alpha State file: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("flush functional alpha State file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close functional alpha State file: %w", err)
	}
	return nil
}
