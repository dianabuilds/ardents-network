package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseSyntaxRefusesBeforeHistoryEffects(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent")
	for _, args := range [][]string{{"verify-initial"}, {"verify-initial", "missing", "pin", root, "invalid-time"}, {"unknown", "missing", "pin", root, "2030-01-02T03:04:05Z"}} {
		var out bytes.Buffer
		if code := runRelease(context.Background(), args, &out); code != 2 {
			t.Fatalf("syntax accepted: %s", out.String())
		}
		if _, err := os.Lstat(root); !os.IsNotExist(err) {
			t.Fatal("invalid syntax opened history")
		}
	}
}
