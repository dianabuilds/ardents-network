package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	return loadJSON(path, bucketName, key, out, false)
}

func LoadJSONStrict(path, bucketName, key string, out any) (found bool, returnErr error) {
	return loadJSON(path, bucketName, key, out, true)
}

func loadJSON(path, bucketName, key string, out any, strict bool) (found bool, returnErr error) {
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
	if !strict {
		if err := json.Unmarshal(payload, out); err != nil {
			return false, err
		}
		return true, nil
	}
	if err := rejectDuplicateJSONFields(payload); err != nil {
		return false, err
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return false, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return false, fmt.Errorf("persisted JSON has trailing content")
	}
	return true, nil
}

func rejectDuplicateJSONFields(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var scan func(string) error
	scan = func(path string) error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]struct{}{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return fmt.Errorf("invalid JSON object key at %s", path)
				}
				if _, duplicate := seen[key]; duplicate {
					return fmt.Errorf("duplicate JSON field at %s.%s", path, key)
				}
				seen[key] = struct{}{}
				if err := scan(path + "." + key); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		case '[':
			for index := 0; decoder.More(); index++ {
				if err := scan(fmt.Sprintf("%s[%d]", path, index)); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		default:
			return fmt.Errorf("invalid JSON delimiter at %s", path)
		}
	}
	if err := scan("$"); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
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
