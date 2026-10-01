//go:build linux

package entry

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// maximumStateBytes bounds one serialized closed Entry set generation.
const maximumStateBytes = 64 << 10

// stateName is the exact content-addressed generation filename grammar.
var stateName = regexp.MustCompile(`^[0-9a-f]{64}$`)

func sha256Hex(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

// readBounded reads one whole file or fails when it exceeds its bound.
func readBounded(path string, maximum int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if int64(len(raw)) > maximum {
		return nil, fmt.Errorf("bounded file exceeds %d bytes", maximum)
	}
	return raw, nil
}

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
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	committed = true
	return syncDirectory(root)
}

func replaceCurrent(root, name string) error {
	return replaceOwnedFile(root, ".current-", "current", []byte(name+"\n"))
}

// replaceOwnedFile preserves Entry's durable same-directory replacement
// mechanics. Callers retain their own value validation and transition order.
func replaceOwnedFile(root, temporaryPrefix, finalName string, raw []byte) error {
	temporary, err := os.CreateTemp(root, temporaryPrefix)
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
	if err := os.Rename(path, filepath.Join(root, finalName)); err != nil {
		return err
	}
	return syncDirectory(root)
}

func cleanupGenerations(root, current, previous string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, dirEntry := range entries {
		if dirEntry.IsDir() || !strings.HasPrefix(dirEntry.Name(), "state-") {
			continue
		}
		name := strings.TrimPrefix(dirEntry.Name(), "state-")
		if name != current && name != previous {
			if err := os.Remove(filepath.Join(root, dirEntry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return syncDirectory(root)
}
