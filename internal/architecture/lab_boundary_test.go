package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func assertLaboratoryQuarantine(t *testing.T, root string) {
	t.Helper()
	namespaceEntries, err := os.ReadDir(filepath.Join(root, "internal", "lab"))
	if err != nil {
		t.Fatalf("read laboratory namespace: %v", err)
	}
	for _, entry := range namespaceEntries {
		if !entry.IsDir() {
			t.Errorf("internal/lab is a namespace and may contain only Module directories: %s", entry.Name())
		}
	}
	required := []string{
		"cmd/carrier-lab",
		"cmd/named-site-lab",
		"internal/lab/carrier",
		"internal/lab/directcontrol",
		"internal/lab/namedsite",
		"internal/lab/nativecircuit",
		"internal/lab/preflight",
		"internal/lab/routecomparison",
		"internal/lab/runlayout",
		"internal/lab/sourceidentity",
		"internal/lab/tooling",
		"lab/carrier",
		"lab/named-site",
	}
	for _, relative := range required {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil || !info.IsDir() {
			t.Errorf("maintained laboratory directory %q is missing", relative)
		}
	}
	obsolete := []string{
		"carrier-lab",
		"cmd/reference-site-lab",
		"internal/directcontrol",
		"internal/experimentidentity",
		"internal/experimentrun",
		"internal/harness",
		"internal/nativecircuit",
		"internal/preflight",
		"internal/routeexperiment",
		"internal/siteexperiment",
		"reference-site",
	}
	for _, relative := range obsolete {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Errorf("obsolete unquarantined laboratory directory still exists: %s", relative)
		}
	}
}
