package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
)

func TestIssuanceCanceledBeforeReadingConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, operation := range []string{"initialize", "inspect-profile"} {
		t.Run(operation, func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			args := []string{"issuance", operation, "--config", filepath.Join(t.TempDir(), "missing")}
			if code := run(ctx, args, &out, &diagnostic); code != 130 {
				t.Fatalf("canceled command returned %d: %s", code, out.String())
			}
		})
	}
}
