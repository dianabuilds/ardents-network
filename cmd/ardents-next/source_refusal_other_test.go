//go:build !linux

package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestSourceUnsupportedPlatformRefusesBeforePlanEffects(t *testing.T) {
	var out, diagnostic bytes.Buffer
	if code := run(t.Context(), []string{"source", "serve", filepath.Join(t.TempDir(), "absent-plan")}, &out, &diagnostic); code != 2 || out.Len() != 0 || diagnostic.Len() != 0 {
		t.Fatal("unsupported Source performed command effects", code, out.String(), diagnostic.String())
	}
}
