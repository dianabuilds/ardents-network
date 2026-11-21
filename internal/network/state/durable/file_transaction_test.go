package durable

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestControlCommitPointerSyncFailureAfterRename(t *testing.T) {
	rootPath := t.TempDir()
	root, err := Open(rootPath, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	previous := fmt.Sprintf("%064x", 1)
	successor := fmt.Sprintf("%064x", 2)
	if err := root.CommitControl(previous, []byte("previous floor")); err != nil {
		t.Fatal(err)
	}
	syncFailure := errors.New("injected directory sync failure")
	renamed := false
	err = root.commitControlWithPointerSync(successor, []byte("successor floor"), func(path string) error {
		if path != filepath.Join(rootPath, "distribution") {
			t.Fatalf("sync path = %q", path)
		}
		raw, readErr := os.ReadFile(filepath.Join(path, "current"))
		if readErr != nil || string(raw) != successor+"\n" {
			t.Fatalf("control pointer before sync = %q, %v", raw, readErr)
		}
		renamed = true
		return syncFailure
	})
	if !renamed || !errors.Is(err, ErrPointerSyncUncertain) || !errors.Is(err, syncFailure) {
		t.Fatalf("post-rename control result: renamed=%t err=%v", renamed, err)
	}
	name, raw, err := root.LoadControl()
	if err != nil || name != successor || string(raw) != "successor floor" {
		t.Fatalf("visible control floor = %q %q, %v", name, raw, err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(rootPath, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	name, raw, err = reopened.LoadControl()
	if err != nil || name != successor || string(raw) != "successor floor" {
		t.Fatalf("reopened control floor = %q %q, %v", name, raw, err)
	}
}

func TestPointerRenameFailureIsNotSyncUncertainty(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "current"), 0o700); err != nil {
		t.Fatal(err)
	}
	syncCalled := false
	err := replacePointerWithSync(root, "current", "successor", func(string) error {
		syncCalled = true
		return nil
	})
	if err == nil || errors.Is(err, ErrPointerSyncUncertain) || syncCalled {
		t.Fatalf("pre-rename result: syncCalled=%t err=%v", syncCalled, err)
	}
}
