//go:build linux && text_worker_installed

package state_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstalledCommandRouteDiagnosticsReportsOnlyFixedReason(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "route-diagnostic.json"), []byte(`{"kind":"route-diagnostic","reason":"issuer-outer-read-eof"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := installedCommandRouteDiagnostics(t, map[string]any{"route_diagnostic_paths": []string{directory}}); got != "issuer-outer-read-eof" {
		t.Fatalf("diagnostics = %q", got)
	}
}
