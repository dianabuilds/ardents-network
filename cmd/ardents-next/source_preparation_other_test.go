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
	if run(context.Background(), []string{"source", "prepare-trust", filepath.Join(root, "missing"), filepath.Join(root, "output")}, &out, &diagnostic) != 2 || out.Len() != 0 || diagnostic.Len() != 0 {
		t.Fatal("unsupported outcome")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("unsupported effects", err)
	}
}
