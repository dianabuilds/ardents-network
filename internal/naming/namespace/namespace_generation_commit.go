package namespace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// commit publishes one immutable Namespace generation and advances current.
func (root *namespaceRoot) commit(generation namespaceGeneration) error {
	return root.commitCurrent("", false, generation)
}

// commitFrom atomically publishes a generation only if the current pointer is
// still expected. EpochInstallation uses it to prevent a stale candidate from
// overwriting a generation published after its verified snapshot.
func (root *namespaceRoot) commitFrom(expected string, generation namespaceGeneration) error {
	return root.commitCurrent(expected, true, generation)
}

func (root *namespaceRoot) commitCurrent(expected string, requireExpected bool, generation namespaceGeneration) error {
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.available(); err != nil {
		return err
	}
	if requireExpected {
		pointer, err := readNamespaceFile(filepath.Join(root.path, "current"), 65)
		if expected == "" && os.IsNotExist(err) {
			// The caller observed an empty current pointer and it is still empty.
		} else if err != nil || string(pointer) != expected+"\n" {
			return errors.New("naming state current generation changed")
		}
	}
	if !namespaceGenerationName.MatchString(generation.Name) || len(generation.Epoch) == 0 ||
		len(generation.Epoch) > maximumNamespaceEpochBytes || len(generation.Inputs) > maximumChunks {
		return errors.New("naming state generation exceeds its bounds")
	}
	for _, input := range generation.Inputs {
		if len(input) == 0 || len(input) > maximumNamespaceRecordBytes {
			return errors.New("naming state generation input exceeds its bound")
		}
	}
	generationsRoot := filepath.Join(root.path, "generations")
	staging, err := os.MkdirTemp(generationsRoot, ".stage-")
	if err != nil {
		return fmt.Errorf("create naming state generation staging: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(staging)
		}
	}()
	if err := writeNamespaceGeneration(staging, generation); err != nil {
		return err
	}
	final := filepath.Join(generationsRoot, generation.Name)
	if info, statErr := os.Stat(final); statErr == nil {
		if !info.IsDir() || !namespaceGenerationMatches(final, generation) {
			return errors.New("existing immutable naming state generation disagrees with supplied bytes")
		}
		if err := os.RemoveAll(staging); err != nil {
			return err
		}
	} else if !os.IsNotExist(statErr) {
		return statErr
	} else if err := os.Rename(staging, final); err != nil {
		return fmt.Errorf("publish naming state generation: %w", err)
	}
	committed = true
	if err := syncNamespaceDirectory(generationsRoot); err != nil {
		return err
	}
	if generation.Activate {
		return replaceNamespacePointer(root.path, "current", generation.Name)
	}
	return nil
}
