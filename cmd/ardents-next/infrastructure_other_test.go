//go:build !linux

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInfrastructureUnsupportedPlatformRefusesBeforeEffects(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent")
	for _, operation := range []string{"prepare-descriptor", "prepare-inventory", "authenticate-initial", "authenticate-candidate"} {
		var out, diagnostic bytes.Buffer
		if code := run(t.Context(), []string{"infrastructure", operation, root}, &out, &diagnostic); code != 1 {
			t.Fatal("unsupported path accepted", operation, code)
		}
		assertUnsupportedPlatformRefusal(t, &out, &diagnostic, "infrastructure")
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Fatal("unsupported path touched input")
		}
	}
}
