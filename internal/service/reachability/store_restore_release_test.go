package reachability

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreRestoreRetainsLeaseReleaseFailure(t *testing.T) {
	config := StoreConfig{Root: t.TempDir(), NetworkID: [32]byte{1}}
	opened, err := OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	badRecord := filepath.Join(config.Root, storeRecords, "invalid")
	if err := os.WriteFile(badRecord, []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	releaseFailure := errors.New("injected lease release failure")
	store, err := openStore(config, func(lease *storeLease) error {
		return errors.Join(lease.release(), releaseFailure)
	})
	if store != nil || !errors.Is(err, releaseFailure) || !strings.Contains(err.Error(), "record name is invalid") {
		t.Fatalf("OpenStore with invalid record and release failure = %v, %v", store, err)
	}
	if err := os.Remove(badRecord); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(config)
	if err != nil {
		t.Fatalf("lease remained held after failed restore: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}
