package blockedentry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitmentInventoryRequiresCanonicalSafeRecords(t *testing.T) {
	root := t.TempDir()
	valid := strings.Repeat("a", 64) + " 7 one.bin\n" + strings.Repeat("b", 64) + " 9 two/key.bin\n"
	path := filepath.Join(root, "inventory.sha256")
	if err := os.WriteFile(path, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateCommitmentInventory(path); err != nil {
		t.Fatalf("canonical inventory rejected: %v", err)
	}
	for name, raw := range map[string]string{
		"traversal":  strings.Repeat("a", 64) + " 7 ../one.bin\n",
		"duplicate":  strings.Repeat("a", 64) + " 7 one.bin\n" + strings.Repeat("b", 64) + " 9 one.bin\n",
		"unsorted":   strings.Repeat("a", 64) + " 7 two.bin\n" + strings.Repeat("b", 64) + " 9 one.bin\n",
		"zero":       strings.Repeat("a", 64) + " 0 one.bin\n",
		"no-newline": strings.Repeat("a", 64) + " 7 one.bin",
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := validateCommitmentInventory(path); err == nil {
				t.Fatal("invalid inventory accepted")
			}
		})
	}
}
