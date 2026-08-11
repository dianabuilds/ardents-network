package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

func readOfflineInputs(raw rawConfig) ([]byte, [][]byte, []byte, error) {
	epoch, err := readCommandFile(raw.epoch, 1<<20)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read epoch: %w", err)
	}
	inputs, err := readInputDirectory(raw.inputs)
	if err != nil {
		return nil, nil, nil, err
	}
	materialBytes, err := readCommandFile(raw.material, 35<<10)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read materialization: %w", err)
	}
	return epoch, inputs, materialBytes, nil
}

func readInputDirectory(path string) ([][]byte, error) {
	directory, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read inputs: %w", err)
	}
	entries, readErr := directory.ReadDir(65)
	closeErr := directory.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, fmt.Errorf("read inputs: %w", readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close inputs: %w", closeErr)
	}
	if len(entries) > 64 {
		return nil, errors.New("input directory exceeds 64 files")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	inputs := make([][]byte, len(entries))
	for index, entry := range entries {
		expected := fmt.Sprintf("%04d.bin", index)
		if entry.IsDir() || entry.Name() != expected {
			return nil, fmt.Errorf("input entry %q is not canonical %q", entry.Name(), expected)
		}
		inputs[index], err = readCommandFile(filepath.Join(path, entry.Name()), 32<<10)
		if err != nil {
			return nil, fmt.Errorf("read input %d: %w", index, err)
		}
	}
	return inputs, nil
}

func readCommandFile(path string, maximum int64) ([]byte, error) {
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
		return nil, errors.New("input file exceeds its framing bound")
	}
	return contents, nil
}
