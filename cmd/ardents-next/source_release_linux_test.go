package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSourceReleaseRefusesMissingInventoryBeforeHistory(t *testing.T) {
	for _, initial := range []bool{true, false} {
		t.Run(map[bool]string{true: "initial", false: "candidate"}[initial], func(t *testing.T) {
			root := t.TempDir()
			bundle := filepath.Join(root, "absent-bundle")
			history := filepath.Join(root, "absent-history")
			args := []string{"authenticate-candidate", bundle, history, "closed", "network", "2030-01-02T03:04:05Z"}
			if initial {
				args = []string{"authenticate-initial", bundle, strings.Repeat("0", 64), history, "closed", "network", "2030-01-02T03:04:05Z"}
			}
			a, v, refused, err := openSourceRelease(t.Context(), args, initial, history, time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC))
			if err == nil || refused != "infrastructure-inventory-refused" || v != nil || a.HasInventory() {
				t.Fatal("missing inventory was accepted", refused, err)
			}
			if _, err := os.Lstat(history); !os.IsNotExist(err) {
				t.Fatal("refusal created Release history", err)
			}
		})
	}
}
