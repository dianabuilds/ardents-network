package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func readMetadataDir(dir string) (result map[string][]byte, resultErr error) {
	before, err := os.Lstat(dir)
	if err != nil {
		return nil, fmt.Errorf("read metadata dir: %w", err)
	}
	if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("metadata dir is not a directory")
	}
	directory, err := os.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("open metadata dir: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, directory.Close()) }()
	after, statErr := directory.Stat()
	if statErr != nil || !after.IsDir() || !os.SameFile(before, after) {
		return nil, errors.Join(errors.New("metadata dir changed while it was opened"), statErr)
	}
	entries := make([]os.DirEntry, 0, maximumMetadataEntries)
	for {
		batch, readErr := directory.ReadDir(8)
		entries = append(entries, batch...)
		if len(entries) > maximumMetadataEntries {
			return nil, errors.New("metadata dir exceeds the entry-count bound")
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("read metadata dir: %w", readErr)
		}
	}
	files := make(map[string][]byte, maximumMetadataFiles)
	var aggregate int64
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if len(files) >= maximumMetadataFiles {
			return nil, errors.New("metadata dir exceeds the file-count bound")
		}
		data, err := readBoundedRegularFile(path, maximumMetadataFileBytes)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		aggregate += int64(len(data))
		if aggregate > maximumMetadataAggregateBytes {
			return nil, errors.New("metadata dir exceeds the aggregate byte bound")
		}
		urlPath := "https://release.invalid/metadata/" + entry.Name()
		files[urlPath] = data
	}
	if len(files) == 0 {
		return nil, errors.New("metadata dir is empty")
	}
	return files, nil
}

const maximumMetadataEntries = 32
const maximumMetadataFiles = 32
const maximumMetadataAggregateBytes = 8 << 20
