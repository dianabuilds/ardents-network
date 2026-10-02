package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvalidInputDoesNotDisclosePathsOrBytes(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "private-permission")
	facts := filepath.Join(dir, "private-facts")
	if err := os.WriteFile(secret, bytes.Repeat([]byte("secret"), 40), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(facts, []byte("private-input"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostic bytes.Buffer
	if code := run(context.Background(), []string{"inspect-permission", secret, facts}, &out, &diagnostic); code != 2 {
		t.Fatalf("exit %d", code)
	}
	if out.String() != "{\"outcome\":\"invalid-input\"}\n" || diagnostic.Len() != 0 {
		t.Fatalf("unexpected output %q / %q", out.String(), diagnostic.String())
	}
}

type unavailableOutput struct{}

func (unavailableOutput) Write([]byte) (int, error) {
	return 0, errors.New("private-output-error")
}

func TestOutputFailureHasFiniteExit(t *testing.T) {
	var diagnostic bytes.Buffer
	if code := run(context.Background(), commandFixture(t), unavailableOutput{}, &diagnostic); code != 2 {
		t.Fatalf("output failure exit %d", code)
	}
	if bytes.Contains(diagnostic.Bytes(), []byte("private-output-error")) {
		t.Fatal("output error disclosed")
	}
}

func TestFactsRejectDuplicateUnknownAndTrailingInput(t *testing.T) {
	for _, raw := range []string{`{"network":"a","network":"b"}`, `{"unknown":"value"}`, `{} {}`, `[]`, `null`} {
		if _, err := decodeFacts([]byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

func TestBoundedInput(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "input")
	if err := os.WriteFile(filename, []byte(strings.Repeat("x", 229)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readBounded(filename, 228); err == nil {
		t.Fatal("oversize accepted")
	}
	if _, err := readBounded(t.TempDir(), 228); err == nil {
		t.Fatal("directory accepted")
	}
}
