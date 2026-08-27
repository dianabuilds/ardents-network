//go:build !windows

package state

import (
	"os"
	"testing"
)

func protectAlphaGenesisTestRoot(t *testing.T, root string) {
	t.Helper()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
}

func weakenAlphaGenesisTestRoot(t *testing.T, root string) {
	t.Helper()
	if err := os.Chmod(root, 0o750); err != nil {
		t.Fatal(err)
	}
}
