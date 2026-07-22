package storage

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
)

func TestStateDirLockIsExclusiveAndReopenable(t *testing.T) {
	dir := t.TempDir()
	first, err := AcquireStateDirLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })

	if _, err := AcquireStateDirLock(dir); err == nil {
		t.Fatal("second state directory lock succeeded")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := AcquireStateDirLock(dir)
	if err != nil {
		t.Fatalf("reacquire state directory lock: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if got := filepath.Base(filepath.Join(dir, stateLockFileName)); got != stateLockFileName {
		t.Fatalf("lock file name = %q", got)
	}
}

func TestLoadJSONStrictRejectsUnknownDuplicateAndTrailingState(t *testing.T) {
	for name, raw := range map[string]string{
		"unknown":   `{"version":1,"extra":true}`,
		"duplicate": `{"version":1,"version":1}`,
		"trailing":  `{"version":1}{"version":1}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.db")
			db, err := bbolt.Open(path, 0o600, nil)
			if err != nil {
				t.Fatal(err)
			}
			err = db.Update(func(tx *bbolt.Tx) error {
				bucket, err := tx.CreateBucket([]byte("test"))
				if err != nil {
					return err
				}
				return bucket.Put([]byte("state"), []byte(raw))
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			var out struct {
				Version uint32 `json:"version"`
			}
			if found, err := LoadJSONStrict(path, "test", "state", &out); err == nil || found {
				t.Fatalf("LoadJSONStrict() = found %v, error %v", found, err)
			}
		})
	}
}

func TestLoadJSONStrictAcceptsExactState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	require.NoError(t, SaveJSON(path, "test", "state", struct {
		Version uint32 `json:"version"`
	}{Version: 2}))
	var out struct {
		Version uint32 `json:"version"`
	}
	found, err := LoadJSONStrict(path, "test", "state", &out)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint32(2), out.Version)
}
