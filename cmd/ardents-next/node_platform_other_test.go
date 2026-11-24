//go:build !linux

package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestNodeUnsupportedPlatformReportsWriteFailure(t *testing.T) {
	for _, route := range [][]string{
		{"node", "forwarding", "--config"},
		{"node", "issuer", "--config"},
		{"node", "descriptor", "--config"},
		{"node", "introduction", "--config"},
		{"node", "join", "--config"},
		{"node", "forwarding", "repair-retained"},
		{"node", "descriptor", "repair-retained"},
		{"node", "introduction", "repair-retained"},
		{"node", "join", "repair-retained"},
	} {
		t.Run(strings.Join(route[1:], "/"), func(t *testing.T) {
			args := append(route, filepath.Join(t.TempDir(), "unavailable-private-input"))
			var diagnostic bytes.Buffer
			if code := run(t.Context(), args, unavailableOutput{}, &diagnostic); code != 2 || diagnostic.Len() != 0 {
				t.Fatalf("failed Node refusal output returned code %d, stderr %q", code, diagnostic.String())
			}
		})
	}
}
