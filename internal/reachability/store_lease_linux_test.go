//go:build linux

package reachability

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestStoreOriginalLeaseLossAndFirstPhysicalRelease(t *testing.T) {
	store, err := OpenStore(StoreConfig{Root: t.TempDir(), Network: [32]byte{1}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.lease.file.Close(); err != nil {
		t.Fatal(err)
	}
	cause := store.Check()
	if cause == nil {
		t.Fatal("lost original lease accepted")
	}
	if err := store.Check(); !errors.Is(err, cause) {
		t.Fatal("first original lease loss replaced", cause, err)
	}
	first := store.Close()
	if first == nil {
		t.Fatal("known physical release failure erased")
	}
	var joined sync.WaitGroup
	for i := 0; i < 8; i++ {
		joined.Add(1)
		go func() {
			defer joined.Done()
			if err := store.Close(); err != first {
				t.Error("physical Close result replaced", err, first)
			}
		}()
	}
	joined.Wait()
}

func TestStoreOriginalLockReplacementCannotRestoreOwner(t *testing.T) {
	root := t.TempDir()
	store, err := OpenStore(StoreConfig{Root: root, Network: [32]byte{1}})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	lock, retained := filepath.Join(root, storeLockName), filepath.Join(root, storeLockName+".retained")
	if err := os.Rename(lock, retained); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lock, nil, 0600); err != nil {
		t.Fatal(err)
	}
	cause := store.Check()
	if cause == nil {
		t.Fatal("replacement lock accepted")
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(retained, lock); err != nil {
		t.Fatal(err)
	}
	if err := store.Check(); !errors.Is(err, cause) {
		t.Fatal("restored lock revived original", err)
	}
	if other, err := OpenStore(StoreConfig{Root: root, Network: [32]byte{1}}); err == nil {
		_ = other.Close()
		t.Fatal("failed original lease not retained")
	}
	if err := store.Close(); err != nil {
		t.Fatal("operational cause became physical release failure", err)
	}
}
