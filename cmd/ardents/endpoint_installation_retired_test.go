package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyEndpointInstallationRetiredBeforeEffects(t *testing.T) {
	root := t.TempDir()
	record := filepath.Join(root, "retained-state")
	original := []byte("retained application data and trust history\n")
	if err := os.WriteFile(record, original, 0600); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"provision", "upgrade-installed", "recover-installed", "start-installed", "installation-check"} {
		for _, args := range [][]string{{"endpoint", operation}, {"endpoint", operation, root}, {"endpoint", operation, filepath.Join(root, "missing"), "extra"}} {
			var out bytes.Buffer
			err := runEndpoint(context.Background(), args, &out)
			if err == nil || !strings.Contains(err.Error(), "legacy Endpoint installation is retired") || out.Len() != 0 {
				t.Fatalf("retired command entered installation: %v %s", err, out.String())
			}
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("retired command changed root: %v %v", entries, err)
	}
	after, err := os.ReadFile(record)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatalf("retired command changed retained data: %v", err)
	}
}
