package custody

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const maximumAlphaStaticFileBytes = 64 << 20

type alphaStaticDirectory struct {
	Generation uint64
	Files      map[string][]byte
}

func alphaStaticFileNames(generation uint64) []string {
	if generation == 0 {
		return nil
	}
	return []string{
		"1.root.json", strconv.FormatUint(generation, 10) + ".snapshot.json", strconv.FormatUint(generation, 10) + ".targets.json",
		"RELEASE", "catalog.ac1", "catalog.pub", "compatibility.ac1", "compatibility.pub", "corpus.pub", "network.ac1", "network.pub",
		"release.ac1", "release.pub", "timestamp.json",
	}
}

func readAlphaStaticDirectory(path string) (alphaStaticDirectory, error) {
	if path == "" || !filepath.IsAbs(path) {
		return alphaStaticDirectory{}, ErrInvalid
	}
	root := filepath.Clean(path)
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return alphaStaticDirectory{}, ErrInvalid
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != len(alphaInputFileNames) {
		return alphaStaticDirectory{}, ErrInvalid
	}
	files := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		item, statErr := os.Lstat(filepath.Join(root, name))
		if statErr != nil || !item.Mode().IsRegular() || item.Mode()&os.ModeSymlink != 0 || item.Size() < 1 || item.Size() > maximumAlphaStaticFileBytes {
			return alphaStaticDirectory{}, ErrInvalid
		}
		contents, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil || int64(len(contents)) != item.Size() {
			return alphaStaticDirectory{}, ErrInvalid
		}
		files[name] = contents
	}
	generation, err := alphaStaticMetadataGeneration(files)
	if err != nil {
		return alphaStaticDirectory{}, err
	}
	return alphaStaticDirectory{Generation: generation, Files: files}, nil
}

func alphaStaticMetadataGeneration(files map[string][]byte) (uint64, error) {
	if len(files) != len(alphaInputFileNames) || len(files["1.root.json"]) == 0 || len(files["timestamp.json"]) == 0 ||
		len(files["RELEASE"]) == 0 || len(files["catalog.ac1"]) == 0 || len(files["catalog.pub"]) == 0 ||
		len(files["release.ac1"]) == 0 || len(files["release.pub"]) == 0 || len(files["network.ac1"]) == 0 ||
		len(files["network.pub"]) == 0 || len(files["compatibility.ac1"]) == 0 || len(files["compatibility.pub"]) == 0 || len(files["corpus.pub"]) == 0 {
		return 0, ErrInvalid
	}
	var snapshot, targets string
	for name := range files {
		switch {
		case strings.HasSuffix(name, ".snapshot.json"):
			if snapshot != "" {
				return 0, ErrInvalid
			}
			snapshot = name
		case strings.HasSuffix(name, ".targets.json"):
			if targets != "" {
				return 0, ErrInvalid
			}
			targets = name
		case name == "1.root.json" || name == "timestamp.json" || name == "RELEASE" || name == "catalog.ac1" || name == "catalog.pub" || name == "release.ac1" || name == "release.pub" || name == "network.ac1" || name == "network.pub" || name == "compatibility.ac1" || name == "compatibility.pub" || name == "corpus.pub":
		default:
			return 0, ErrInvalid
		}
	}
	if snapshot == "" || targets == "" {
		return 0, ErrInvalid
	}
	version := strings.TrimSuffix(snapshot, ".snapshot.json")
	if version == "" || version != strings.TrimSuffix(targets, ".targets.json") || strings.HasPrefix(version, "0") {
		return 0, ErrInvalid
	}
	generation, err := strconv.ParseUint(version, 10, 64)
	if err != nil || generation == 0 {
		return 0, fmt.Errorf("%w: static metadata generation", ErrInvalid)
	}
	for _, name := range alphaStaticFileNames(generation) {
		if len(files[name]) == 0 {
			return 0, ErrInvalid
		}
	}
	return generation, nil
}
