package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOperatorInputIsBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readOperatorInput(path, 1); !errors.Is(err, errOperatorInputTooLarge) {
		t.Fatalf("bounded input error = %v", err)
	}
}
