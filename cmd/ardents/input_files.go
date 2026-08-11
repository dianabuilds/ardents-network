package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/dianabuilds/ardents-network/internal/networkstate"
)

func readOfflineInputs(raw rawConfig) ([]byte, [][]byte, networkstate.Materialization, error) {
	epoch, err := readCommandFile(raw.epoch, 1<<20)
	if err != nil {
		return nil, nil, networkstate.Materialization{}, fmt.Errorf("read epoch: %w", err)
	}
	inputs, err := readInputDirectory(raw.inputs)
	if err != nil {
		return nil, nil, networkstate.Materialization{}, err
	}
	materialBytes, err := readCommandFile(raw.material, 35<<10)
	if err != nil {
		return nil, nil, networkstate.Materialization{}, fmt.Errorf("read materialization: %w", err)
	}
	material, err := decodeMaterialization(materialBytes)
	return epoch, inputs, material, err
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

func decodeMaterialization(raw []byte) (networkstate.Materialization, error) {
	const fixed = 32 + 4 + 4 + 2
	if len(raw) < fixed {
		return networkstate.Materialization{}, errors.New("materialization is truncated")
	}
	var material networkstate.Materialization
	copy(material.EpochDigest[:], raw[:32])
	material.Index = binary.BigEndian.Uint32(raw[32:36])
	recordLength := int(binary.BigEndian.Uint32(raw[36:40]))
	if recordLength < 1 || recordLength > 32<<10 || len(raw) < 40+recordLength+2 {
		return networkstate.Materialization{}, errors.New("materialization record length is invalid")
	}
	material.Record = append([]byte(nil), raw[40:40+recordLength]...)
	offset := 40 + recordLength
	count := int(binary.BigEndian.Uint16(raw[offset : offset+2]))
	offset += 2
	if count > 64 || len(raw) != offset+count*32 {
		return networkstate.Materialization{}, errors.New("materialization proof framing is invalid")
	}
	material.Siblings = make([][32]byte, count)
	for index := range material.Siblings {
		copy(material.Siblings[index][:], raw[offset+index*32:offset+(index+1)*32])
	}
	return material, nil
}
