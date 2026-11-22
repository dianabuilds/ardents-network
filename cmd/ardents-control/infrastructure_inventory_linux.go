package main

import (
	"errors"
	"io"
	"path/filepath"
	"strings"
)

// readInfrastructureInventory retains the protected originals in inputs.
// Its returned check reobserves both the original files and the exact directory
// inventory at the caller's output boundaries.
func (in *sourceControlInputs) readInfrastructureInventory(distribution, destination, program, template, descriptor string) (map[string][]byte, func() error, error) {
	if !sourceControlPath(distribution) || distribution == destination || strings.HasPrefix(destination, distribution+"/") {
		return nil, nil, errSourceControlInput
	}
	directory, err := in.directory(distribution)
	if err != nil {
		return nil, nil, err
	}
	wanted := map[string]int64{
		program:          64 << 20,
		template:         64 << 20,
		descriptor:       16 << 10,
		"root.json":      1 << 20,
		"timestamp.json": 1 << 20,
		"snapshot.json":  1 << 20,
		"targets.json":   1 << 20,
	}
	checkNames := func() error {
		if _, err := directory.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		entries, err := directory.file.ReadDir(8)
		if err != nil && err != io.EOF || len(entries) != 7 {
			return errSourceControlInput
		}
		for _, entry := range entries {
			if _, ok := wanted[entry.Name()]; !ok {
				return errSourceControlInput
			}
		}
		return nil
	}
	if err := checkNames(); err != nil {
		return nil, nil, err
	}
	files := make(map[string][]byte, len(wanted))
	for name, maximum := range wanted {
		raw, err := in.read(filepath.Join(distribution, name), maximum)
		if err != nil {
			return nil, nil, err
		}
		files[name] = raw
	}
	check := func() error { return errors.Join(in.checkProtected(), checkNames()) }
	return files, check, nil
}
