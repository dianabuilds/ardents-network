//go:build !linux

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallationCheckRefusesUnsupportedPlatformBeforeFileEffects(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent-installation")
	var output bytes.Buffer
	err := runEndpoint(context.Background(), []string{"endpoint", "installation-check", root}, &output)
	if err == nil || !strings.Contains(err.Error(), "requires Ubuntu24.04 amd64") || output.Len() != 0 {
		t.Fatalf("unsupported installation observation produced a receipt: %v / %q", err, output.String())
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("unsupported observation changed root: %v", err)
	}
}
