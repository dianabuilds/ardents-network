package planfile_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/planfile"
)

func TestFreshRegularFailsClosed(t *testing.T) {
	now := time.Now().UTC()
	path := filepath.Join(t.TempDir(), "confidence")
	if err := os.WriteFile(path, []byte("observed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatal(err)
	}
	check := planfile.FreshRegular(path, func() time.Time { return now }, 2*time.Second)
	if !check() {
		t.Fatal("fresh regular file was rejected")
	}
	if err := os.Chtimes(path, now.Add(-time.Minute), now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if check() || planfile.FreshRegular("", time.Now, 2*time.Second)() {
		t.Fatal("missing or stale file was accepted")
	}
}
