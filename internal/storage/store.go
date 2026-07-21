package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.etcd.io/bbolt"
)

const dbFileName = "ardents.db"

func PathInDir(dir string) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, dbFileName)
}

func LoadJSON(path, bucketName, key string, out any) (found bool, returnErr error) {
	if path == "" {
		return false, nil
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if err := ProtectPrivateFile(path); err != nil {
		return false, err
	}

	db, err := bbolt.Open(path, 0o600, nil)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := db.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close storage database: %w", err))
		}
	}()

	var payload []byte
	if err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		if bucket == nil {
			return nil
		}
		value := bucket.Get([]byte(key))
		if value == nil {
			return nil
		}
		payload = append([]byte(nil), value...)
		return nil
	}); err != nil {
		return false, err
	}
	if len(payload) == 0 {
		return false, nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return false, err
	}
	return true, nil
}

func SaveJSON(path, bucketName, key string, value any) (returnErr error) {
	if path == "" {
		return nil
	}
	if err := EnsurePrivateDir(filepath.Dir(path)); err != nil {
		return err
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}

	db, err := bbolt.Open(path, 0o600, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close storage database: %w", err))
		}
	}()
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	if err := ProtectPrivateFile(path); err != nil {
		return err
	}

	return db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return err
		}
		return bucket.Put([]byte(key), payload)
	})
}
