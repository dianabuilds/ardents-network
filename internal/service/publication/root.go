package publication

import (
	"bytes"
	"crypto"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	rootMarkerName = ".ardents-service-publication-v1"
	rootMarker     = "ardents-service-publication-v1\n"
	rootLockName   = ".ardents-service-publication-lock"
	floorName      = "floor"
	currentName    = "current"
)

var publicationGenerationName = regexp.MustCompile(`^[0-9a-f]{16}$`)

// durableRoot serializes one publication root and keeps all mutable root
// checks under its lock. The lease excludes a second process owner.
type durableRoot struct {
	mu      sync.Mutex
	path    string
	lease   rootLease
	closed  bool
	floor   uint64
	current *generation
}

type generation struct {
	credential Credential
	record     []byte
	digest     [32]byte
	signer     crypto.Signer
	release    func()
	refs       uint32
	withdrawn  bool
	drained    chan struct{}
}

func (generation *generation) releaseSigner() {
	if generation == nil || generation.signer == nil {
		return
	}
	if generation.release != nil {
		generation.release()
	}
	generation.signer, generation.release = nil, nil
}

func openDurableRoot(config Config) (*durableRoot, error) {
	if config.Root == "" {
		return nil, errors.New("publication root is required")
	}
	path, err := filepath.Abs(config.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve publication root: %w", err)
	}
	if err := inspectRoot(path); err != nil {
		return nil, err
	}
	if err := ensureLeasePath(path); err != nil {
		return nil, err
	}
	lease, err := acquireRootLease(path)
	if err != nil {
		return nil, err
	}
	opened := false
	defer func() {
		if !opened {
			_ = lease.release()
		}
	}()
	if err := prepareRoot(path); err != nil {
		return nil, err
	}
	root := &durableRoot{path: path, lease: lease}
	if err := root.restore(config); err != nil {
		return nil, err
	}
	opened = true
	return root, nil
}

func inspectRoot(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("create publication root: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect publication root: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("publication root is not an owned directory")
	}
	return nil
}

func prepareRoot(path string) error {
	markerPath := filepath.Join(path, rootMarkerName)
	marker, err := os.Lstat(markerPath)
	if errors.Is(err, os.ErrNotExist) {
		entries, scanErr := readDirectory(path, 2)
		if scanErr != nil || len(entries) != 1 || entries[0].Name() != rootLockName {
			return errors.New("refusing to claim a non-empty unowned publication root")
		}
		if err := writeExclusive(markerPath, []byte(rootMarker)); err != nil {
			return err
		}
	} else if err != nil || !marker.Mode().IsRegular() || marker.Mode()&os.ModeSymlink != 0 {
		return errors.New("publication root ownership marker is invalid")
	} else if contents, readErr := readFile(markerPath, int64(len(rootMarker))); readErr != nil || !bytes.Equal(contents, []byte(rootMarker)) {
		return errors.New("publication root ownership marker is invalid")
	}
	if err := os.Mkdir(filepath.Join(path, "generations"), 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("create publication generations: %w", err)
	}
	return cleanupStaging(path)
}

func ensureLeasePath(path string) error {
	lockPath := filepath.Join(path, rootLockName)
	if _, err := os.Lstat(lockPath); errors.Is(err, os.ErrNotExist) {
		file, createErr := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if createErr != nil {
			return fmt.Errorf("create publication root lease: %w", createErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			return fmt.Errorf("close publication root lease: %w", closeErr)
		}
	} else if err != nil {
		return fmt.Errorf("inspect publication root lease: %w", err)
	}
	return nil
}

func (root *durableRoot) restore(config Config) error {
	entries, err := readDirectory(filepath.Join(root.path, "generations"), 2)
	if err != nil {
		return fmt.Errorf("scan publication generations: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || !publicationGenerationName.MatchString(entry.Name()) {
			return errors.New("publication root contains an invalid generation")
		}
	}
	floor, floorExists, err := readFloor(root.path)
	if err != nil {
		return err
	}
	if !floorExists {
		if len(entries) != 0 || currentExists(root.path) {
			return errors.New("publication root lacks its monotonic floor")
		}
		legacy, legacyErr := readLegacyFloor(config.LegacyFloor)
		if legacyErr != nil {
			return legacyErr
		}
		floor = legacy
		if floor != 0 && writeFloor(root.path, floor) != nil {
			return errors.New("publication root cannot persist migrated floor")
		}
	}
	root.floor = floor
	pointer, pointerExists, err := readPointer(root.path)
	if err != nil {
		return err
	}
	if !pointerExists {
		if len(entries) == 0 {
			return nil
		}
		if len(entries) == 1 && entries[0].Name() == publicationGeneration(floor) {
			generation, loadErr := loadGeneration(root.path, entries[0].Name(), config)
			if loadErr == nil && generation.credential.Generation == floor {
				return removeGeneration(root.path, floor)
			}
		}
		return errors.New("publication root has a generation without its current pointer")
	}
	if len(entries) != 1 || entries[0].Name() != pointer {
		return errors.New("publication root has surplus or mismatched generations")
	}
	generation, err := loadGeneration(root.path, pointer, config)
	if err != nil || generation.credential.Generation != floor {
		return errors.New("publication pointer, floor, or record is inconsistent")
	}
	// A public record can survive a restart, but its private key cannot. It is
	// deliberately unavailable until a newer live generation is published.
	return nil
}

func (root *durableRoot) removeCurrent() error {
	if err := os.Remove(filepath.Join(root.path, currentName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("withdraw publication pointer: %w", err)
	}
	return nil
}

func readFloor(root string) (uint64, bool, error) {
	raw, err := readFile(filepath.Join(root, floorName), 21)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > 20 || strings.TrimSpace(string(raw)) != string(raw[:len(raw)-1]) {
		return 0, false, errors.New("publication floor is malformed")
	}
	value, parseErr := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	if parseErr != nil || value == 0 {
		return 0, false, errors.New("publication floor is malformed")
	}
	return value, true, nil
}

func readLegacyFloor(path string) (uint64, error) {
	if path == "" {
		return 0, nil
	}
	raw, err := readFile(path, 21)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil || len(raw) == 0 || len(raw) > 20 {
		return 0, errors.New("legacy publication floor is malformed")
	}
	value, parseErr := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	if parseErr != nil {
		return 0, errors.New("legacy publication floor is malformed")
	}
	return value, nil
}

func writeFloor(root string, floor uint64) error {
	return replaceFile(root, floorName, []byte(strconv.FormatUint(floor, 10)+"\n"))
}

func currentExists(root string) bool {
	_, err := os.Lstat(filepath.Join(root, currentName))
	return err == nil
}

func readPointer(root string) (string, bool, error) {
	raw, err := readFile(filepath.Join(root, currentName), 18)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	name := strings.TrimSuffix(string(raw), "\n")
	if err != nil || string(raw) != name+"\n" || !publicationGenerationName.MatchString(name) {
		return "", false, errors.New("publication current pointer is malformed")
	}
	return name, true, nil
}

func replacePointer(root, generation string) error {
	return replaceFile(root, currentName, []byte(generation+"\n"))
}

func generationPath(root, name string) string { return filepath.Join(root, "generations", name) }

func cleanupStaging(root string) error {
	for _, directory := range []string{root, filepath.Join(root, "generations")} {
		entries, err := readDirectory(directory, 128)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".stage-") || strings.HasPrefix(entry.Name(), ".current-") {
				if err := os.RemoveAll(filepath.Join(directory, entry.Name())); err != nil {
					return fmt.Errorf("remove interrupted publication staging: %w", err)
				}
			}
		}
	}
	return nil
}

func readFile(path string, maximum int64) ([]byte, error) {
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
		return nil, errors.New("publication file exceeds its bound")
	}
	return contents, nil
}

func readDirectory(path string, maximum int) ([]os.DirEntry, error) {
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
		return nil, errors.New("publication directory exceeds its entry bound")
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Name() < entries[right].Name() })
	return entries, nil
}

func writeExclusive(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		_, err = file.Write(contents)
	}
	if err == nil {
		err = file.Sync()
	}
	return errors.Join(err, file.Close())
}

func replaceFile(root, name string, contents []byte) error {
	temporary, err := os.CreateTemp(root, ".current-")
	if err != nil {
		return err
	}
	path := temporary.Name()
	defer func() { _ = os.Remove(path) }()
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(contents)
	}
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(path, filepath.Join(root, name))
	}
	return err
}
