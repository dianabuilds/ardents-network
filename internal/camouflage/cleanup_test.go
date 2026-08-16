package camouflage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateCleanupIsBoundedAndLeavesOverBoundTreeForHarness(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	for index := range 33 {
		path := filepath.Join(root, fmt.Sprintf("entry-%02d", index))
		if err := os.WriteFile(path, []byte{1}, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := removeAndVerifyState(root, time.Now().Add(time.Second)); err == nil {
		t.Fatal("over-bound state tree was treated as clean")
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("failed cleanup erased evidence needed by the harness: %v", err)
	}
	bounded := filepath.Join(t.TempDir(), "bounded")
	if err := os.Mkdir(bounded, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bounded, "entry"), []byte{1}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := removeAndVerifyState(bounded, time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(bounded); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("bounded state residue: %v", err)
	}
}
