package enrollment

import (
	"encoding/hex"
	"errors"
	"os"
	"strings"
)

// readEnrollmentFile admits only an owned bounded regular file from either a
// Portable bundle or its explicitly package-owned static directory.
func readEnrollmentFile(path string, packageStatic bool) ([]byte, error) {
	if !packageStatic {
		return readRegular(path)
	}
	return readPackageStatic(path)
}

func readRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 1 || info.Size() > maximumFileLen {
		return nil, errors.New("entry is not a bounded regular file")
	}
	if err := verifyOwnedRegular(info); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func readPackageArtifact(path string) ([]byte, error) {
	return readPackageStatic(path)
}

func readPackageStatic(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 1 || info.Size() > maximumFileLen {
		return nil, errors.New("package artifact is not a bounded regular file")
	}
	if err := verifyPackageFile(info); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func verifyPackageStaticRoot(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("package static root is not a direct directory")
	}
	return verifyPackageDirectory(info)
}

func parseManifest(raw []byte) (map[string][]byte, error) {
	if len(raw) == 0 || len(raw) > maximumFiles*80 || raw[len(raw)-1] != '\n' {
		return nil, errors.New("alpha manifest is not canonical")
	}
	result := make(map[string][]byte)
	previous := ""
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		parts := strings.Split(line, "  ")
		if len(parts) != 2 || len(parts[0]) != 64 || !validName(parts[1]) || parts[1] == manifestName || previous >= parts[1] {
			return nil, errors.New("alpha manifest is not canonical")
		}
		digest, err := hex.DecodeString(parts[0])
		if err != nil || strings.ToLower(parts[0]) != parts[0] {
			return nil, errors.New("alpha manifest digest is invalid")
		}
		if _, exists := result[parts[1]]; exists || len(result) == maximumFiles {
			return nil, errors.New("alpha manifest inventory is invalid")
		}
		result[parts[1]], previous = digest, parts[1]
	}
	if _, found := result[descriptorName]; !found {
		return nil, errors.New("alpha manifest lacks its descriptor")
	}
	return result, nil
}

func exactInventory(root string, entries map[string][]byte, artifact string, externalArtifact bool) error {
	directory, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	want := len(entries) + 1
	if externalArtifact {
		want--
	}
	if len(directory) != want {
		return errors.New("alpha bundle inventory has unknown or missing entries")
	}
	for _, entry := range directory {
		if entry.Name() == manifestName {
			continue
		}
		if externalArtifact && entry.Name() == artifact {
			return errors.New("alpha bundle inventory unexpectedly contains packaged artifact")
		}
		if _, found := entries[entry.Name()]; !found {
			return errors.New("alpha bundle inventory has an unknown entry")
		}
	}
	return nil
}
