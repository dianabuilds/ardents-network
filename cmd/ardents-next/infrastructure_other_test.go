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
	for _, args := range [][]string{
		{"prepare-runtime-descriptor", "source", "release", "1", root, root, root},
		{"prepare-runtime-inventory", "source", root, root},
		{"authenticate-initial", root},
		{"authenticate-candidate", root},
	} {
		var out, diagnostic bytes.Buffer
		if code := run(t.Context(), append([]string{"infrastructure"}, args...), &out, &diagnostic); code != 1 {
			t.Fatal("unsupported path accepted", args[0], code)
		}
		assertUnsupportedPlatformRefusal(t, &out, &diagnostic, "infrastructure")
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Fatal("unsupported path touched input")
		}
	}
}
