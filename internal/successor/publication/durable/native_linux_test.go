package durable

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeOwnedRootReopen(t *testing.T) {
	config := Config{Root: filepath.Join(t.TempDir(), "parent", "publication"), Target: [32]byte{1}, Network: [32]byte{2}}
	root, err := Open(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Open(context.Background(), config); err == nil {
		t.Fatal("native lease reused")
	}
	if err = root.Close(); err != nil {
		t.Fatal(err)
	}
	if err = exclusive(filepath.Join(config.Root, "floor"), []byte("18446744073709551615\n")); err != nil {
		t.Fatal(err)
	}
	if err = syncDirectory(config.Root); err != nil {
		t.Fatal(err)
	}
	root, err = Open(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	floor, err := root.Floor(context.Background())
	if err != nil || floor != ^uint64(0) {
		t.Fatal(floor, err)
	}
	if err = os.Rename(config.Root, config.Root+"-original"); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(config.Root, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = root.Floor(context.Background()); err == nil {
		t.Fatal("substituted root supplied original floor")
	}
}

func TestNativeCloseRetainsReleaseFailure(t *testing.T) {
	config := Config{Root: filepath.Join(t.TempDir(), "publication"), Target: [32]byte{1}, Network: [32]byte{2}}
	root, err := Open(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	// Mechanical physical failure control, not a successful cleanup receipt.
	if err = root.lease.file.Close(); err != nil {
		t.Fatal(err)
	}
	first := root.Close()
	if first == nil {
		t.Fatal("physical release failure hidden")
	}
	if again := root.Close(); again != first || !errors.Is(again, os.ErrClosed) {
		t.Fatalf("terminal release failure changed: %v -> %v", first, again)
	}
}

func TestNativeLeaseReplacementRefusesFloor(t *testing.T) {
	config := Config{Root: filepath.Join(t.TempDir(), "publication"), Target: [32]byte{1}, Network: [32]byte{2}}
	root, err := Open(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	lock := filepath.Join(config.Root, lockName)
	if err = os.Rename(lock, lock+"-original"); err != nil {
		t.Fatal(err)
	}
	if err = exclusive(lock, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = root.Floor(context.Background()); err == nil {
		t.Fatal("detached original lease supplied floor")
	}
}
