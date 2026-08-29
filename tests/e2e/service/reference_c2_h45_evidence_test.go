//go:build h4_5_rendezvous

package service_test

import (
	"os"
	"path/filepath"
	"testing"
)

func h45PrepareEvidenceRoot(t *testing.T, root string) {
	t.Helper()
	if root == "" || !filepath.IsAbs(root) {
		t.Fatal("H4-5 evidence root must be absolute")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatalf("prepare H4-5 evidence root: %v", err)
	}
}
