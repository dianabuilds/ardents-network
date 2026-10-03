//go:build linux

package hosting

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHostingRefusesIndependentBudgetRootWithoutEffects(t *testing.T) {
	root := filepath.Join(t.TempDir(), "budget")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"budget.pin", "budget.lock", "budget.json"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("independent budget bytes"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if owner, err := Open(root); err == nil {
		owner.Close()
		t.Fatal("legacy opened independent root")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 3 {
		t.Fatalf("foreign root mutated: %v %v", entries, err)
	}
	for _, entry := range entries {
		raw, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil || string(raw) != "independent budget bytes" {
			t.Fatalf("foreign bytes changed %v", err)
		}
	}
}
