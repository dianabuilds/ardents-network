package namestore

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

// Cohesion note: this file keeps one Namespace-compatible durable-root
// implementation together: exclusive ownership, bounded reads, immutable
// generations, and atomic activation form one filesystem transaction. Splitting
// it would expose its private layout across helpers. Store restart, tamper, and
// partial-batch behavior tests cover that local invariant.

const (
	namespaceRootMarkerName     = ".ardents-network-state-v1"
	namespaceRootMarker         = "ardents-network-state-v1\n"
	namespaceRootLockName       = ".ardents-network-state-lock"
	maximumNamespaceGenerations = 64
	maximumNamespaceEpochBytes  = 1 << 20
	maximumNamespaceRecordBytes = 32 << 10
)

var namespaceGenerationName = regexp.MustCompile(`^[0-9a-f]{64}$`)

// namespaceRoot owns the Namespace-local compatible durable representation.
// The marker and layout remain unchanged so this ownership transfer is not a
// format migration.
type namespaceRoot struct {
	mu     sync.Mutex
	path   string
	lease  namespaceRootLease
	closed bool
}

type namespaceGeneration struct {
	Name     string
	Epoch    []byte
	Inputs   [][]byte
	Activate bool
}

func openNamespaceRoot(path string) (*namespaceRoot, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := inspectNamespaceRoot(absolute); err != nil {
		return nil, err
	}
	lease, err := acquireNamespaceRootLease(absolute)
	if err != nil {
		return nil, err
	}
	opened := false
	defer func() {
		if !opened {
			_ = lease.release()
		}
	}()
	if err := prepareNamespaceRoot(absolute); err != nil {
		return nil, err
	}
	if err := verifyNamespaceRootWritable(absolute); err != nil {
		return nil, err
	}
	root := &namespaceRoot{path: absolute, lease: lease}
	if err := prepareNamespaceControl(root.path); err != nil {
		return nil, err
	}
	opened = true
	return root, nil
}

func (root *namespaceRoot) close() error {
	root.mu.Lock()
	defer root.mu.Unlock()
	if root.closed {
		return nil
	}
	root.closed = true
	return root.lease.release()
}

func (root *namespaceRoot) available() error {
	if root.closed {
		return errors.New("naming state store is closed")
	}
	return nil
}

func (root *namespaceRoot) load() (string, []namespaceGeneration, error) {
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.available(); err != nil {
		return "", nil, err
	}
	generationsRoot := filepath.Join(root.path, "generations")
	entries, err := readNamespaceDirectory(generationsRoot, maximumNamespaceGenerations)
	if err != nil {
		return "", nil, fmt.Errorf("scan naming state generations: %w", err)
	}
	generations := make([]namespaceGeneration, 0, len(entries))
	known := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !namespaceGenerationName.MatchString(entry.Name()) {
			return "", nil, errors.New("naming state generation directory is invalid")
		}
		generation, loadErr := loadNamespaceGeneration(generationsRoot, entry.Name())
		if loadErr != nil {
			return "", nil, loadErr
		}
		known[entry.Name()] = true
		generations = append(generations, generation)
	}
	pointer, err := readNamespaceFile(filepath.Join(root.path, "current"), 65)
	if os.IsNotExist(err) {
		return "", generations, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("read current naming state pointer: %w", err)
	}
	current := strings.TrimSuffix(string(pointer), "\n")
	if string(pointer) != current+"\n" || !namespaceGenerationName.MatchString(current) || !known[current] {
		return "", nil, errors.New("current naming state pointer is invalid")
	}
	return current, generations, nil
}

func (root *namespaceRoot) commit(generation namespaceGeneration) error {
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.available(); err != nil {
		return err
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

func loadNamespaceGeneration(root, name string) (namespaceGeneration, error) {
	directory := filepath.Join(root, name)
	epoch, err := readNamespaceFile(filepath.Join(directory, "epoch.bin"), maximumNamespaceEpochBytes)
	if err != nil {
		return namespaceGeneration{}, fmt.Errorf("read naming state generation epoch: %w", err)
	}
	inputsRoot := filepath.Join(directory, "inputs")
	entries, err := readNamespaceDirectory(inputsRoot, maximumChunks)
	if err != nil {
		return namespaceGeneration{}, fmt.Errorf("scan naming state generation inputs: %w", err)
	}
	inputs := make([][]byte, len(entries))
	for index, entry := range entries {
		if entry.IsDir() || entry.Name() != fmt.Sprintf("%04d.bin", index) {
			return namespaceGeneration{}, errors.New("naming state generation input name is not canonical")
		}
		inputs[index], err = readNamespaceFile(filepath.Join(inputsRoot, entry.Name()), maximumNamespaceRecordBytes)
		if err != nil {
			return namespaceGeneration{}, fmt.Errorf("read naming state generation input: %w", err)
		}
	}
	return namespaceGeneration{Name: name, Epoch: epoch, Inputs: inputs}, nil
}

func writeNamespaceGeneration(directory string, generation namespaceGeneration) error {
	inputsRoot := filepath.Join(directory, "inputs")
	if err := os.Mkdir(inputsRoot, 0o700); err != nil {
		return err
	}
	if err := writeNamespaceFile(filepath.Join(directory, "epoch.bin"), generation.Epoch); err != nil {
		return err
	}
	for index, input := range generation.Inputs {
		if err := writeNamespaceFile(filepath.Join(inputsRoot, fmt.Sprintf("%04d.bin", index)), input); err != nil {
			return err
		}
	}
	if err := syncNamespaceDirectory(inputsRoot); err != nil {
		return err
	}
	return syncNamespaceDirectory(directory)
}

func namespaceGenerationMatches(directory string, generation namespaceGeneration) bool {
	actual, err := loadNamespaceGeneration(filepath.Dir(directory), filepath.Base(directory))
	if err != nil || !bytes.Equal(actual.Epoch, generation.Epoch) || len(actual.Inputs) != len(generation.Inputs) {
		return false
	}
	for index := range actual.Inputs {
		if !bytes.Equal(actual.Inputs[index], generation.Inputs[index]) {
			return false
		}
	}
	return true
}

func inspectNamespaceRoot(root string) error {
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(root, 0o700); err != nil {
			return fmt.Errorf("create naming state root: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("inspect naming state root: %w", err)
	} else if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("naming state root is not an owned directory")
	}
	markerInfo, markerErr := os.Lstat(filepath.Join(root, namespaceRootMarkerName))
	if markerErr == nil {
		if !markerInfo.Mode().IsRegular() || markerInfo.Mode()&os.ModeSymlink != 0 {
			return errors.New("naming state root ownership marker is not a regular file")
		}
		return nil
	}
	if !os.IsNotExist(markerErr) {
		return fmt.Errorf("inspect naming state root ownership marker: %w", markerErr)
	}
	entries, readErr := readNamespaceDirectory(root, 2)
	if readErr != nil || len(entries) > 1 || len(entries) == 1 && entries[0].Name() != namespaceRootLockName {
		return errors.New("refusing to claim a non-empty unowned naming state root")
	}
	return nil
}

func prepareNamespaceRoot(root string) error {
	if err := ensureNamespaceRootMarker(root); err != nil {
		return err
	}
	generations := filepath.Join(root, "generations")
	if err := os.MkdirAll(generations, 0o700); err != nil {
		return fmt.Errorf("create naming state generations directory: %w", err)
	}
	if err := cleanupNamespaceStaging(root, generations); err != nil {
		return err
	}
	return syncNamespaceDirectory(root)
}

func ensureNamespaceRootMarker(root string) error {
	markerPath := filepath.Join(root, namespaceRootMarkerName)
	info, err := os.Lstat(markerPath)
	if err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("naming state root ownership marker is not a regular file")
		}
		contents, readErr := readNamespaceFile(markerPath, int64(len(namespaceRootMarker)))
		if readErr != nil || !bytes.Equal(contents, []byte(namespaceRootMarker)) {
			return errors.New("naming state root ownership marker is invalid")
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("inspect naming state root ownership marker: %w", err)
	}
	entries, readErr := readNamespaceDirectory(root, 2)
	if readErr != nil || len(entries) != 1 || entries[0].Name() != namespaceRootLockName {
		return errors.New("refusing to claim a non-empty unowned naming state root")
	}
	if err := writeNamespaceFile(markerPath, []byte(namespaceRootMarker)); err != nil {
		return fmt.Errorf("create naming state root ownership marker: %w", err)
	}
	return syncNamespaceDirectory(root)
}

func cleanupNamespaceStaging(root, generations string) error {
	for directory, prefix := range map[string]string{root: ".current-", generations: ".stage-"} {
		entries, err := readNamespaceDirectory(directory, 128)
		if err != nil {
			return fmt.Errorf("scan owned naming state root: %w", err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), prefix) {
				if err := os.RemoveAll(filepath.Join(directory, entry.Name())); err != nil {
					return fmt.Errorf("remove interrupted owned naming state %q: %w", entry.Name(), err)
				}
			}
		}
	}
	return nil
}

func prepareNamespaceControl(root string) error {
	directory := filepath.Join(root, "distribution")
	generations := filepath.Join(directory, "generations")
	if err := os.MkdirAll(generations, 0o700); err != nil {
		return fmt.Errorf("create naming state control journal: %w", err)
	}
	if err := cleanupNamespaceControlStaging(directory, generations); err != nil {
		return err
	}
	if err := syncNamespaceDirectory(generations); err != nil {
		return err
	}
	if err := syncNamespaceDirectory(directory); err != nil {
		return err
	}
	return syncNamespaceDirectory(root)
}

func cleanupNamespaceControlStaging(root, generations string) error {
	for directory, prefix := range map[string]string{root: ".current-", generations: ".stage-"} {
		entries, err := readNamespaceDirectory(directory, 4098)
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

func verifyNamespaceRootWritable(root string) error {
	probe, err := os.CreateTemp(root, ".current-durability-")
	if err != nil {
		return fmt.Errorf("create naming state durability probe: %w", err)
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
		return fmt.Errorf("complete naming state durability probe: %w", err)
	}
	if err := syncNamespaceDirectory(root); err != nil {
		return fmt.Errorf("sync naming state durability probe: %w", err)
	}
	return nil
}

func readNamespaceFile(path string, maximum int64) ([]byte, error) {
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

func readNamespaceDirectory(path string, maximum int) ([]os.DirEntry, error) {
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

func writeNamespaceFile(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create immutable naming state file: %w", err)
	}
	if _, err = file.Write(contents); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("write immutable naming state file: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close immutable naming state file: %w", closeErr)
	}
	return nil
}

func replaceNamespacePointer(root, name, generation string) error {
	temporary, err := os.CreateTemp(root, ".current-")
	if err != nil {
		return fmt.Errorf("create naming state pointer staging: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.WriteString(generation + "\n")
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return fmt.Errorf("write naming state pointer staging: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close naming state pointer staging: %w", closeErr)
	}
	if err := os.Rename(temporaryPath, filepath.Join(root, name)); err != nil {
		return fmt.Errorf("replace naming state pointer: %w", err)
	}
	if err := syncNamespaceDirectory(root); err != nil {
		return fmt.Errorf("sync naming state pointer: %w", err)
	}
	return nil
}
