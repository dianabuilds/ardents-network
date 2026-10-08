package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInstalledEndpointCancelledBeforeEffects(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent-installation")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var output, diagnostic bytes.Buffer
	if code := run(ctx, []string{"endpoint", "start-installed", root}, &output, &diagnostic); code != 1 {
		t.Fatal("cancelled installed opening accepted", code)
	}
	if output.Len() != 0 || diagnostic.Len() != 0 {
		t.Fatal("refusal reported participant readiness")
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("refusal changed installation", err)
	}
	for _, args := range [][]string{{"endpoint"}, {"endpoint", "start-installed"}, {"endpoint", "foreign", root}} {
		if code := run(t.Context(), args, &output, &diagnostic); code != 2 {
			t.Fatal("malformed installed selector admitted", args, code)
		}
	}
}
