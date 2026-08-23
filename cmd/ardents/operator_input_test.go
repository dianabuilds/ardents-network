package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOperatorInputIsBoundedAndFreshnessFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readOperatorInput(path, 1); !errors.Is(err, errOperatorInputTooLarge) {
		t.Fatalf("bounded input error = %v", err)
	}
	now := time.Now().UTC()
	check := freshOperatorRegularFile(path, func() time.Time { return now }, time.Minute)
	if !check() {
		t.Fatal("fresh regular operator input was rejected")
	}
	if freshOperatorRegularFile("", time.Now, time.Minute)() {
		t.Fatal("empty operator input passed freshness")
	}
	if freshOperatorRegularFile(path, nil, time.Minute)() {
		t.Fatal("operator input with no clock passed freshness")
	}
}
