package state

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

var canonicalGeneration = regexp.MustCompile(`^[0-9a-f]{64}$`)

type persistedEvidence struct {
	current          string
	generations      map[string][]byte
	inputs           [][]byte
	materializations [][]byte
}

func readEvidence(root string, materializations [][]byte) (persistedEvidence, error) {
	pointer, err := byteio.ReadFile(filepath.Join(root, "current"), 65)
	if err != nil {
		return persistedEvidence{}, fmt.Errorf("read candidate current pointer: %w", err)
	}
	current := strings.TrimSuffix(string(pointer), "\n")
	if string(pointer) != current+"\n" || !canonicalGeneration.MatchString(current) {
		return persistedEvidence{}, errors.New("candidate current pointer is not canonical")
	}
	generations, err := readGenerations(root)
	if err != nil {
		return persistedEvidence{}, err
	}
	inputs, err := readCurrentInputs(root, current)
	if err != nil {
		return persistedEvidence{}, err
	}
	return persistedEvidence{current: current, generations: generations,
		inputs: inputs, materializations: materializations}, nil
}

func readGenerations(root string) (map[string][]byte, error) {
	directory := filepath.Join(root, "generations")
	entries, err := byteio.ReadDirectory(directory, 64)
	if err != nil || len(entries) == 0 {
		return nil, errors.Join(errors.New("read candidate generations"), err)
	}
	generations := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !canonicalGeneration.MatchString(entry.Name()) {
			return nil, errors.New("candidate generation directory is not canonical")
		}
		raw, readErr := byteio.ReadFile(filepath.Join(directory, entry.Name(), "epoch.bin"), 1<<20)
		if readErr != nil {
			return nil, fmt.Errorf("read candidate generation %s: %w", entry.Name(), readErr)
		}
		generations[entry.Name()] = raw
	}
	return generations, nil
}

func readCurrentInputs(root, current string) ([][]byte, error) {
	directory := filepath.Join(root, "generations", current, "inputs")
	entries, err := byteio.ReadDirectory(directory, 64)
	if err != nil {
		return nil, fmt.Errorf("read candidate inputs: %w", err)
	}
	names := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			return nil, errors.New("candidate input entry is not a file")
		}
		names[entry.Name()] = true
	}
	inputs := make([][]byte, len(entries))
	for index := range inputs {
		name := fmt.Sprintf("%04d.bin", index)
		if !names[name] {
			return nil, errors.New("candidate input filenames are not canonical")
		}
		inputs[index], err = byteio.ReadFile(filepath.Join(directory, name), 32<<10)
		if err != nil {
			return nil, fmt.Errorf("read candidate input %d: %w", index, err)
		}
	}
	return inputs, nil
}
