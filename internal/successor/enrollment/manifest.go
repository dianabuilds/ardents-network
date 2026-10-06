package enrollment

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maximumFiles   = 32
	maximumFileLen = 64 << 20
)

func canonicalDigest(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func matchesDigest(raw []byte, value string) bool {
	digest := sha256.Sum256(raw)
	return canonicalDigest(value) && hex.EncodeToString(digest[:]) == value
}

func validName(name string) bool {
	return name != "" && name != "." && name != ".." && filepath.Base(name) == name &&
		!strings.ContainsAny(name, "\\/\t\r\n :\x00")
}

func parseManifest(raw []byte) (map[string]string, error) {
	if len(raw) == 0 || len(raw) > maximumFiles*80 || raw[len(raw)-1] != '\n' {
		return nil, ErrInventory
	}
	entries := make(map[string]string)
	previous := ""
	for _, line := range strings.Split(string(raw[:len(raw)-1]), "\n") {
		parts := strings.Split(line, "  ")
		if len(parts) != 2 || !canonicalDigest(parts[0]) || !validName(parts[1]) ||
			parts[1] == "SHA256SUMS" || previous >= parts[1] || len(entries) == maximumFiles {
			return nil, ErrInventory
		}
		entries[parts[1]], previous = parts[0], parts[1]
	}
	if _, ok := entries["RELEASE"]; !ok {
		return nil, ErrInventory
	}
	return entries, nil
}

func sortedEntries(entries map[string]string) []string {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
