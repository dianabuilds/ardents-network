package state

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	maximumStateGenerations = 64
)

// durableGeneration is one opaque immutable State generation. Its meaning is
// verified by the State acceptance path before it becomes current.
type durableGeneration struct {
	Name     string
	Epoch    []byte
	Inputs   [][]byte
	Activate bool
}

func (root *durableRoot) loadState() (string, []durableGeneration, error) {
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.available(); err != nil {
		return "", nil, err
	}
	generationsRoot := filepath.Join(root.path, "generations")
	entries, err := readBoundedDirectory(generationsRoot, maximumStateGenerations)
	if err != nil {
		return "", nil, fmt.Errorf("scan state generations: %w", err)
	}
	generations := make([]durableGeneration, 0, len(entries))
	known := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !generationName.MatchString(entry.Name()) {
			return "", nil, errors.New("state generation directory is invalid")
		}
		generation, loadErr := loadStateGeneration(generationsRoot, entry.Name())
		if loadErr != nil {
			return "", nil, loadErr
		}
		known[entry.Name()] = true
		generations = append(generations, generation)
	}
	pointer, err := readBoundedFile(filepath.Join(root.path, "current"), 65)
	if os.IsNotExist(err) {
		return "", generations, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("read current state pointer: %w", err)
	}
	current := strings.TrimSuffix(string(pointer), "\n")
	if string(pointer) != current+"\n" || !generationName.MatchString(current) || !known[current] {
		return "", nil, errors.New("current state pointer is invalid")
	}
	return current, generations, nil
}

func (root *durableRoot) commitState(generation durableGeneration) error {
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.available(); err != nil {
		return err
	}
	if !generationName.MatchString(generation.Name) || len(generation.Epoch) == 0 ||
		len(generation.Epoch) > maximumEpochBytes || len(generation.Inputs) > 64 {
		return errors.New("state generation exceeds its bounds")
	}
	for _, input := range generation.Inputs {
		if len(input) == 0 || len(input) > maximumRecordBytes {
			return errors.New("state generation input exceeds its bound")
		}
	}
	generationsRoot := filepath.Join(root.path, "generations")
	staging, err := os.MkdirTemp(generationsRoot, ".stage-")
	if err != nil {
		return fmt.Errorf("create state generation staging: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(staging)
		}
	}()
	if err := writeStateGeneration(staging, generation); err != nil {
		return err
	}
	final := filepath.Join(generationsRoot, generation.Name)
	if info, statErr := os.Stat(final); statErr == nil {
		if !info.IsDir() || !stateGenerationMatches(final, generation) {
			return errors.New("existing immutable state generation disagrees with supplied bytes")
		}
		if err := os.RemoveAll(staging); err != nil {
			return err
		}
	} else if !os.IsNotExist(statErr) {
		return statErr
	} else if err := os.Rename(staging, final); err != nil {
		return fmt.Errorf("publish state generation: %w", err)
	}
	committed = true
	if err := syncDirectory(generationsRoot); err != nil {
		return err
	}
	if generation.Activate {
		return replacePointer(root.path, "current", generation.Name)
	}
	return nil
}

func loadStateGeneration(root, name string) (durableGeneration, error) {
	directory := filepath.Join(root, name)
	epoch, err := readBoundedFile(filepath.Join(directory, "epoch.bin"), maximumEpochBytes)
	if err != nil {
		return durableGeneration{}, fmt.Errorf("read state generation Epoch: %w", err)
	}
	inputsRoot := filepath.Join(directory, "inputs")
	entries, err := readBoundedDirectory(inputsRoot, 64)
	if err != nil {
		return durableGeneration{}, fmt.Errorf("scan state generation inputs: %w", err)
	}
	inputs := make([][]byte, len(entries))
	for index, entry := range entries {
		if entry.IsDir() || entry.Name() != fmt.Sprintf("%04d.bin", index) {
			return durableGeneration{}, errors.New("state generation input name is not canonical")
		}
		inputs[index], err = readBoundedFile(filepath.Join(inputsRoot, entry.Name()), maximumRecordBytes)
		if err != nil {
			return durableGeneration{}, fmt.Errorf("read state generation input: %w", err)
		}
	}
	return durableGeneration{Name: name, Epoch: epoch, Inputs: inputs}, nil
}

func writeStateGeneration(directory string, generation durableGeneration) error {
	inputsRoot := filepath.Join(directory, "inputs")
	if err := os.Mkdir(inputsRoot, 0o700); err != nil {
		return err
	}
	if err := writeSynced(filepath.Join(directory, "epoch.bin"), generation.Epoch); err != nil {
		return err
	}
	for index, input := range generation.Inputs {
		if err := writeSynced(filepath.Join(inputsRoot, fmt.Sprintf("%04d.bin", index)), input); err != nil {
			return err
		}
	}
	if err := syncDirectory(inputsRoot); err != nil {
		return err
	}
	return syncDirectory(directory)
}

func stateGenerationMatches(directory string, generation durableGeneration) bool {
	actual, err := loadStateGeneration(filepath.Dir(directory), filepath.Base(directory))
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
