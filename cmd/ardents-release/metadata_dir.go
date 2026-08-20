package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// readMetadataDir reads every regular file in dir and indexes it under
// the canonical offline-import URL key.
func readMetadataDir(dir string) (map[string][]byte, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read metadata dir: %w", err)
	}
	files := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := dir + string(os.PathSeparator) + entry.Name()
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		urlPath := "https://release.invalid/metadata/" + entry.Name()
		files[urlPath] = data
	}
	if len(files) == 0 {
		return nil, errors.New("metadata dir is empty")
	}
	return files, nil
}
