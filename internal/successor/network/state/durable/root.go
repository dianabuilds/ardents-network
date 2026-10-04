package durable

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

	// The root retains a marker, lease, generations directory, distribution
	// directory, current pointer, and two ClosedProfile files per generation.
	maximumRootStableEntries = 5 + 2*maximumStateGenerations
	// Interrupted transactions can accumulate across crashes. Recovery scans a
	// finite number before removing only the staging names it owns.
	maximumOwnedStagingEntries = maximumStateGenerations
)

var generationName = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Root owns the only writer lease and physical State-root transaction.
type Root struct {
	limits   Limits
	mu       sync.Mutex
	path     string
	lease    rootLease
	closed   bool
	closeErr error
}

func Open(path string, limits Limits) (openedRoot *Root, resultErr error) {
	if limits.EpochBytes <= 0 || limits.RecordBytes <= 0 || limits.ClosedProfileBytes <= 0 {
		return nil, errors.New("state storage bounds are invalid")
	}
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
			if releaseErr := lease.release(); releaseErr != nil {
				resultErr = errors.Join(resultErr, releaseErr)
			}
		}
	}()
	if err := prepareRoot(absolute); err != nil {
		return nil, err
	}
	if err := verifyRootWritable(absolute); err != nil {
		return nil, err
	}
	root := &Root{path: absolute, lease: lease, limits: limits}
	if err := root.prepareControl(); err != nil {
		return nil, err
	}
	opened = true
	return root, nil
}

// Close releases the root lease once and retains its terminal result.
func (root *Root) Close() error {
	root.mu.Lock()
	defer root.mu.Unlock()
	if root.closed {
		return root.closeErr
	}
	root.closed = true
	root.closeErr = root.lease.release()
	return root.closeErr
}

func (root *Root) available() error {
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
	if info, err := os.Lstat(generations); os.IsNotExist(err) {
		if err := os.Mkdir(generations, 0o700); err != nil {
			return fmt.Errorf("create generations directory: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("inspect generations directory: %w", err)
	} else if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("state generations root is not an owned directory")
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
	rootStaging, err := scanRootStaging(root)
	if err != nil {
		return fmt.Errorf("scan owned state root: %w", err)
	}
	generationStaging, err := scanGenerationStaging(generations)
	if err != nil {
		return fmt.Errorf("scan state generations root: %w", err)
	}
	for _, name := range rootStaging {
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			return fmt.Errorf("remove interrupted owned state %q: %w", name, err)
		}
	}
	for _, name := range generationStaging {
		if err := os.RemoveAll(filepath.Join(generations, name)); err != nil {
			return fmt.Errorf("remove interrupted state generation %q: %w", name, err)
		}
	}
	if len(generationStaging) != 0 {
		return syncDirectory(generations)
	}
	return nil
}

func scanRootStaging(root string) ([]string, error) {
	entries, err := readBoundedDirectory(root, maximumRootStableEntries+maximumOwnedStagingEntries)
	if err != nil {
		return nil, err
	}
	stable := 0
	staging := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, ".current-") && !strings.HasPrefix(name, ".closed-profile-") {
			stable++
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("inspect staging %q: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("owned staging %q is not a regular file", name)
		}
		staging = append(staging, name)
	}
	if stable > maximumRootStableEntries || len(staging) > maximumOwnedStagingEntries {
		return nil, errors.New("state root exceeds its entry bound")
	}
	return staging, nil
}

func scanGenerationStaging(generations string) ([]string, error) {
	entries, err := readBoundedDirectory(generations, maximumStateGenerations+maximumOwnedStagingEntries)
	if err != nil {
		return nil, err
	}
	committed := 0
	staging := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, ".stage-") {
			committed++
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("inspect generation staging %q: %w", name, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("generation staging %q is not a directory", name)
		}
		staging = append(staging, name)
	}
	if committed > maximumStateGenerations || len(staging) > maximumOwnedStagingEntries {
		return nil, errors.New("state generations root exceeds its entry bound")
	}
	return staging, nil
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
