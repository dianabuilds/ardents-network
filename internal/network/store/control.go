package store

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maximumControlGenerations = 4096

func (root *Root) prepareControl() error {
	directory := filepath.Join(root.path, "distribution")
	generations := filepath.Join(directory, "generations")
	if err := os.MkdirAll(generations, 0o700); err != nil {
		return fmt.Errorf("create control journal: %w", err)
	}
	if err := cleanupStaging(directory, generations); err != nil {
		return err
	}
	if err := syncDirectory(generations); err != nil {
		return err
	}
	if err := syncDirectory(directory); err != nil {
		return err
	}
	return syncDirectory(root.path)
}

// LoadControl returns the current opaque security-control generation. Empty
// values mean that no control state has been committed.
func (root *Root) LoadControl() (string, []byte, error) {
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.available(); err != nil {
		return "", nil, err
	}
	directory := filepath.Join(root.path, "distribution")
	generations := filepath.Join(directory, "generations")
	pointer, err := readBoundedFile(filepath.Join(directory, "current"), 65)
	if os.IsNotExist(err) {
		entries, scanErr := readBoundedDirectory(generations, maximumControlGenerations+1)
		if scanErr != nil || len(entries) != 0 {
			return "", nil, errors.New("control journal lacks its current pointer")
		}
		return "", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	name := strings.TrimSuffix(string(pointer), "\n")
	if string(pointer) != name+"\n" || !generationName.MatchString(name) {
		return "", nil, errors.New("control journal pointer is invalid")
	}
	raw, err := readBoundedFile(filepath.Join(generations, name, "state.bin"), 4096)
	if err != nil {
		return "", nil, err
	}
	return name, raw, nil
}

// CommitControl publishes and activates one bounded opaque control generation.
func (root *Root) CommitControl(name string, raw []byte) error {
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.available(); err != nil {
		return err
	}
	if !generationName.MatchString(name) || len(raw) == 0 || len(raw) > 4096 {
		return errors.New("control generation is invalid")
	}
	directory := filepath.Join(root.path, "distribution")
	generations := filepath.Join(directory, "generations")
	if err := pruneSupersededControl(directory, generations); err != nil {
		return err
	}
	entries, err := readBoundedDirectory(generations, maximumControlGenerations+1)
	if err != nil || len(entries) >= maximumControlGenerations {
		return errors.New("control generation bound is exhausted")
	}
	final := filepath.Join(generations, name)
	if existing, readErr := readBoundedFile(filepath.Join(final, "state.bin"), 4096); readErr == nil {
		if !bytes.Equal(existing, raw) {
			return errors.New("existing immutable control generation disagrees with supplied bytes")
		}
		return replacePointer(directory, "current", name)
	} else if !os.IsNotExist(readErr) {
		return readErr
	}
	staging, err := os.MkdirTemp(generations, ".stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if err := writeSynced(filepath.Join(staging, "state.bin"), raw); err != nil {
		return err
	}
	if err := syncDirectory(staging); err != nil {
		return err
	}
	if err := os.Rename(staging, final); err != nil {
		return fmt.Errorf("publish control generation: %w", err)
	}
	if err := syncDirectory(generations); err != nil {
		return err
	}
	return replacePointer(directory, "current", name)
}

func pruneSupersededControl(directory, generations string) error {
	pointer, err := readBoundedFile(filepath.Join(directory, "current"), 65)
	if os.IsNotExist(err) {
		entries, scanErr := readBoundedDirectory(generations, maximumControlGenerations+1)
		if scanErr != nil || len(entries) != 0 {
			return errors.New("control journal without a pointer contains generations")
		}
		return nil
	}
	if err != nil {
		return err
	}
	current := strings.TrimSuffix(string(pointer), "\n")
	if string(pointer) != current+"\n" || !generationName.MatchString(current) {
		return errors.New("control journal pointer is invalid")
	}
	entries, err := readBoundedDirectory(generations, maximumControlGenerations+1)
	if err != nil {
		return err
	}
	found, changed := false, false
	for _, entry := range entries {
		if !entry.IsDir() || !generationName.MatchString(entry.Name()) {
			return errors.New("control journal contains an invalid generation")
		}
		if entry.Name() == current {
			found = true
			continue
		}
		if err := os.RemoveAll(filepath.Join(generations, entry.Name())); err != nil {
			return err
		}
		changed = true
	}
	if !found {
		return errors.New("control journal current generation is missing")
	}
	if changed {
		return syncDirectory(generations)
	}
	return nil
}

func cleanupStaging(root, generations string) error {
	for directory, prefix := range map[string]string{root: ".current-", generations: ".stage-"} {
		entries, err := readBoundedDirectory(directory, maximumControlGenerations+2)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), prefix) {
				if err := os.RemoveAll(filepath.Join(directory, entry.Name())); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
