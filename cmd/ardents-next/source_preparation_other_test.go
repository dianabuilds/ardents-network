//go:build !linux

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSourcePreparationUnsupportedBeforeEffects(t *testing.T) {
	root := t.TempDir()
	var out, diagnostic bytes.Buffer
	if code := run(context.Background(), []string{"source", "prepare-trust", filepath.Join(root, "missing"), filepath.Join(root, "output")}, &out, &diagnostic); code != 1 {
		t.Fatal("unsupported outcome", code)
	}
	assertUnsupportedPlatformRefusal(t, &out, &diagnostic, "source.prepare-trust")
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("unsupported effects", err)
	}
}
