package selection

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/durablefile"
)

// maximumStateBytes bounds one serialized closed Entry set generation.
const maximumStateBytes = 64 << 10

// stateName is the exact content-addressed generation filename grammar.
var stateName = durablefile.GenerationName

func sha256Hex(raw []byte) string {
	return durablefile.SHA256Hex(raw)
}

// readBounded reads one whole file or fails when it exceeds its bound.
func readBounded(path string, maximum int64) ([]byte, error) {
	return durablefile.ReadBounded(path, maximum, fmt.Sprintf("bounded file exceeds %d bytes", maximum))
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
	return durablefile.SyncDirectory(root)
}

func replaceCurrent(root, name string) error {
	return replaceOwnedFile(root, ".current-", "current", []byte(name+"\n"))
}

// replaceOwnedFile preserves Entry's durable same-directory replacement
// mechanics. Callers retain their own value validation and transition order.
func replaceOwnedFile(root, temporaryPrefix, finalName string, raw []byte) error {
	return durablefile.ReplaceFile(root, temporaryPrefix, finalName, raw)
}

func cleanupGenerations(root, current, previous string) error {
	return durablefile.CleanupGenerations(root, current, previous)
}
