package architecture

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func assertRepositoryContainsNoArtifacts(t *testing.T, root string) {
	t.Helper()
	forbiddenDirectories := map[string]bool{
		".artifacts": true, ".cache": true, ".tmp": true, "build": true,
		"coverage": true, "dist": true, "evidence": true, "node_modules": true,
		"target": true, "tmp": true, "var": true, "vendor": true,
	}
	forbiddenSuffixes := []string{".db", ".db-shm", ".db-wal", ".out", ".pcap", ".pcapng", ".prof", ".test"}
	walk(t, root, func(path string, entry os.DirEntry) {
		relative := relativePath(t, root, path)
		if entry.IsDir() && forbiddenDirectories[entry.Name()] {
			t.Errorf("generated or sensitive directory is forbidden: %s", relative)
		}
		if entry.IsDir() {
			return
		}
		lowerName := strings.ToLower(entry.Name())
		if (entry.Name() == ".env" || strings.HasPrefix(entry.Name(), ".env.")) && entry.Name() != ".env.example" {
			t.Errorf("generated or sensitive file is forbidden: %s", relative)
		}
		if strings.HasSuffix(lowerName, ".key") {
			t.Errorf("key material is forbidden: %s", relative)
		}
		for _, suffix := range forbiddenSuffixes {
			if strings.HasSuffix(lowerName, suffix) {
				t.Errorf("generated or sensitive file is forbidden: %s", relative)
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read repository file %s: %v", relative, err)
			return
		}
		trimmed := bytes.TrimSpace(data)
		if strings.HasSuffix(lowerName, ".pem") &&
			(!pathHasSegment(relative, "testdata") ||
				(!bytes.HasPrefix(trimmed, []byte("-----BEGIN CERTIFICATE-----")) &&
					!bytes.HasPrefix(trimmed, []byte("-----BEGIN PUBLIC KEY-----"))) ||
				bytes.Contains(trimmed, []byte("PRIVATE KEY"))) {
			t.Errorf("PEM files must be owned public-certificate or public-key test fixtures: %s", relative)
		}
	})
}

func pathHasSegment(path, wanted string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(path), "/") {
		if segment == wanted {
			return true
		}
	}
	return false
}
