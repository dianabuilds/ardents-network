package state

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

const (
	rootMarkerName = ".ardents-network-state-v1"
	rootMarker     = "ardents-network-state-v1\n"
	rootLockName   = ".ardents-network-state-lock"
)

var generationName = regexp.MustCompile(`^[0-9a-f]{64}$`)

// durableRoot owns the only writer lease and physical State-root transaction.
type durableRoot struct {
	mu     sync.Mutex
	path   string
	lease  rootLease
	closed bool
}

func openDurableRoot(path string) (*durableRoot, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := inspectRoot(absolute); err != nil {
		return nil, err
	}
	lease, err := acquireRootLease(absolute)
	if err != nil {
		return nil, err
	}
	opened := false
	defer func() {
		if !opened {
			_ = lease.release()
		}
	}()
	if err := prepareRoot(absolute); err != nil {
		return nil, err
	}
	if err := verifyRootWritable(absolute); err != nil {
		return nil, err
	}
	root := &durableRoot{path: absolute, lease: lease}
	if err := root.prepareControl(); err != nil {
		return nil, err
	}
	opened = true
	return root, nil
}

func (root *durableRoot) close() error {
	root.mu.Lock()
	defer root.mu.Unlock()
	if root.closed {
		return nil
	}
	root.closed = true
	return root.lease.release()
}

func (root *durableRoot) available() error {
	if root.closed {
		return errors.New("network state store is closed")
	}
	return nil
}

func inspectRoot(root string) error {
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(root, 0o700); err != nil {
			return fmt.Errorf("create state root: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("inspect state root: %w", err)
	} else if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("state root is not an owned directory")
	}
	markerInfo, markerErr := os.Lstat(filepath.Join(root, rootMarkerName))
	if markerErr == nil {
		if !markerInfo.Mode().IsRegular() || markerInfo.Mode()&os.ModeSymlink != 0 {
			return errors.New("state root ownership marker is not a regular file")
		}
		return nil
	}
	if !os.IsNotExist(markerErr) {
		return fmt.Errorf("inspect state root ownership marker: %w", markerErr)
	}
	entries, readErr := readBoundedDirectory(root, 2)
	if readErr != nil || len(entries) > 1 || len(entries) == 1 && entries[0].Name() != rootLockName {
		return errors.New("refusing to claim a non-empty unowned state root")
	}
	return nil
}

func prepareRoot(root string) error {
	if err := ensureRootMarker(root); err != nil {
		return err
	}
	generations := filepath.Join(root, "generations")
	if err := os.MkdirAll(generations, 0o700); err != nil {
		return fmt.Errorf("create generations directory: %w", err)
	}
	if err := cleanupOwnedStaging(root, generations); err != nil {
		return err
	}
	return syncDirectory(root)
}

func ensureRootMarker(root string) error {
	markerPath := filepath.Join(root, rootMarkerName)
	info, err := os.Lstat(markerPath)
	if err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("state root ownership marker is not a regular file")
		}
		contents, readErr := readBoundedFile(markerPath, int64(len(rootMarker)))
		if readErr != nil || !bytes.Equal(contents, []byte(rootMarker)) {
			return errors.New("state root ownership marker is invalid")
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("inspect state root ownership marker: %w", err)
	}
	entries, readErr := readBoundedDirectory(root, 2)
	if readErr != nil || len(entries) != 1 || entries[0].Name() != rootLockName {
		return errors.New("refusing to claim a non-empty unowned state root")
	}
	if err := writeSynced(markerPath, []byte(rootMarker)); err != nil {
		return fmt.Errorf("create state root ownership marker: %w", err)
	}
	return syncDirectory(root)
}

func cleanupOwnedStaging(root, generations string) error {
	for directory, prefix := range map[string]string{root: ".current-", generations: ".stage-"} {
		entries, err := readBoundedDirectory(directory, 128)
		if err != nil {
			return fmt.Errorf("scan owned state root: %w", err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), prefix) {
				if err := os.RemoveAll(filepath.Join(directory, entry.Name())); err != nil {
					return fmt.Errorf("remove interrupted owned state %q: %w", entry.Name(), err)
				}
			}
		}
	}
	return nil
}

func verifyRootWritable(root string) error {
	probe, err := os.CreateTemp(root, ".current-durability-")
	if err != nil {
		return fmt.Errorf("create state durability probe: %w", err)
	}
	path := probe.Name()
	_, writeErr := probe.Write(make([]byte, 4096))
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

func readBoundedFile(path string, maximum int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	contents, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if int64(len(contents)) > maximum {
		return nil, errors.New("file exceeds its framing bound")
	}
	return contents, nil
}

func readBoundedDirectory(path string, maximum int) ([]os.DirEntry, error) {
	directory, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	entries, readErr := directory.ReadDir(maximum + 1)
	closeErr := directory.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(entries) > maximum {
		return nil, errors.New("directory exceeds its entry bound")
	}
	sort.Slice(entries, func(first, second int) bool { return entries[first].Name() < entries[second].Name() })
	return entries, nil
}
