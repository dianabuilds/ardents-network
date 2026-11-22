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

func TestRetiredPermissionInspectorUsesUnknownCommandRefusal(t *testing.T) {
	var out, diagnostic bytes.Buffer
	args := []string{"inspect-permission", "private-permission", "private-facts"}
	if code := run(context.Background(), args, &out, &diagnostic); code != 2 ||
		out.Len() != 0 || diagnostic.String() != errDocumentUsage.Error()+"\n" {
		t.Fatalf("retired command: code %d, stdout %q, stderr %q", code, out.String(), diagnostic.String())
	}
}

type unavailableOutput struct{}

func (unavailableOutput) Write([]byte) (int, error) {
	return 0, errors.New("private-output-error")
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
	if raw, err := readBounded(filename, 229); err != nil || string(raw) != strings.Repeat("x", 229) {
		t.Fatal("exact bound refused or bytes changed", err)
	}
	if _, err := readBounded(t.TempDir(), 228); err == nil {
		t.Fatal("directory accepted")
	}
}
