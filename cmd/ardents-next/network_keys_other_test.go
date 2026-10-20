//go:build !linux

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInitialKeysUnsupportedBeforeEffects(t *testing.T) {
	parent := t.TempDir()
	plan, destination := filepath.Join(parent, "missing-plan"), filepath.Join(parent, "keys")
	var out bytes.Buffer
	if runNetwork(context.Background(), []string{"prepare-initial-keys", plan, destination}, &out) == 0 || out.Len() != 0 {
		t.Fatal("unsupported command accepted")
	}
	if _, e := os.Stat(destination); !os.IsNotExist(e) {
		t.Fatal("native effect", e)
	}
}
