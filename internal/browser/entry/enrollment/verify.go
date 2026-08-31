package enrollment

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Verify authenticates exactly one enrollment-v4 inventory and returns only
// its Application-owned Browser companions.
func Verify(request Request) (Verified, error) {
	if !validRequest(request) {
		return Verified{}, errors.New("browser enrollment request is incomplete")
	}
	root, err := filepath.Abs(request.BundleRoot)
	if err != nil {
		return Verified{}, errors.New("resolve browser enrollment bundle")
	}
	manifest, err := readRegular(filepath.Join(root, manifestName))
	if err != nil {
		return Verified{}, fmt.Errorf("read browser enrollment manifest: %w", err)
	}
	if !equalDigest(manifest, request.Pin.ManifestSHA256) {
		return Verified{}, errors.New("browser enrollment manifest does not match the independent pin")
	}
	entries, err := parseManifest(manifest)
	if err != nil {
		return Verified{}, err
	}
	files, err := readExactInventory(root, entries)
	if err != nil {
		return Verified{}, err
	}
	descriptor, err := parseDescriptor(files[descriptorName])
	if err != nil {
		return Verified{}, err
	}
	if !descriptor.matches(request) {
		return Verified{}, errors.New("browser enrollment descriptor does not match the request")
	}
	for _, name := range descriptor.required {
		if len(files[name]) == 0 {
			return Verified{}, fmt.Errorf("browser enrollment lacks required entry %q", name)
		}
	}
	if err := exactExecutable(request.ExecutablePath, filepath.Join(root, descriptor.artifact), files[descriptor.artifact]); err != nil {
		return Verified{}, err
	}
	return Verified{BrowserAdapterArtifactName: descriptor.browserAdapter, BrowserAdapterArtifact: append([]byte(nil), files[descriptor.browserAdapter]...),
		BrowserEntryArtifactName: descriptor.browserEntry, BrowserEntryArtifact: append([]byte(nil), files[descriptor.browserEntry]...),
		BrowserEntryExtensionName: descriptor.browserExtension, BrowserEntryExtension: append([]byte(nil), files[descriptor.browserExtension]...)}, nil
}

func validRequest(request Request) bool {
	return request.BundleRoot != "" && request.ExecutablePath != "" && request.Pin.Cohort != "" &&
		request.Pin.Release != "" && request.Pin.Platform != "" && len(request.Pin.ManifestSHA256) == 64 &&
		request.Environment != "" && request.Network != "" && request.TargetPath != "" && request.Architecture != "" &&
		!request.ReferenceTime.IsZero()
}

func readExactInventory(root string, entries map[string][]byte) (map[string][]byte, error) {
	directory, err := os.ReadDir(root)
	if err != nil || len(directory) != len(entries)+1 {
		return nil, errors.New("browser enrollment inventory has unknown or missing entries")
	}
	files := make(map[string][]byte, len(entries))
	for _, entry := range directory {
		if entry.Name() == manifestName {
			continue
		}
		expected, found := entries[entry.Name()]
		if !found {
			return nil, errors.New("browser enrollment inventory has an unknown entry")
		}
		contents, readErr := readRegular(filepath.Join(root, entry.Name()))
		if readErr != nil || !bytes.Equal(digest(contents), expected) {
			return nil, fmt.Errorf("browser enrollment entry %q does not match manifest", entry.Name())
		}
		files[entry.Name()] = contents
	}
	return files, nil
}

func parseManifest(raw []byte) (map[string][]byte, error) {
	if len(raw) == 0 || len(raw) > maximumFiles*80 || raw[len(raw)-1] != '\n' {
		return nil, errors.New("browser enrollment manifest is not canonical")
	}
	result := make(map[string][]byte)
	previous := ""
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		parts := strings.Split(line, "  ")
		if len(parts) != 2 || len(parts[0]) != 64 || !validName(parts[1]) || parts[1] == manifestName || previous >= parts[1] || len(result) == maximumFiles {
			return nil, errors.New("browser enrollment manifest is not canonical")
		}
		value, err := hex.DecodeString(parts[0])
		if err != nil || strings.ToLower(parts[0]) != parts[0] {
			return nil, errors.New("browser enrollment manifest digest is invalid")
		}
		result[parts[1]], previous = value, parts[1]
	}
	if _, found := result[descriptorName]; !found {
		return nil, errors.New("browser enrollment manifest lacks its descriptor")
	}
	return result, nil
}

func readRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 1 || info.Size() > maximumFileLen {
		return nil, errors.New("browser enrollment entry is not a bounded regular file")
	}
	if err := verifyOwnedRegular(info); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func equalDigest(data []byte, expected string) bool {
	decoded, err := hex.DecodeString(expected)
	return err == nil && strings.ToLower(expected) == expected && bytes.Equal(digest(data), decoded)
}

func digest(data []byte) []byte {
	value := sha256.Sum256(data)
	return value[:]
}

func validName(value string) bool {
	return value != "" && filepath.Base(value) == value && !strings.ContainsAny(value, "\\/\t\r\n ")
}
