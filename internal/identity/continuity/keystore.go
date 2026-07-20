package continuity

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	db "ardents/internal/persistence"
)

type FileKeyStore struct {
	path string
}

type keyLedger struct {
	PrivateKey string `json:"private_key"`
}

func NewKeyStoreInDir(dir string) *FileKeyStore {
	return &FileKeyStore{path: filepath.Join(dir, "identity_key.json")}
}

func (s *FileKeyStore) Load() (string, error) {
	if s == nil || s.path == "" {
		return "", nil
	}
	raw, found, err := db.ReadPrivateFile(s.path)
	if err != nil {
		return "", err
	}
	if !found {
		return "", nil
	}
	if len(raw) == 0 {
		return "", fmt.Errorf("identity key file is empty")
	}
	var stored keyLedger
	if err := json.Unmarshal(raw, &stored); err != nil {
		return "", err
	}
	return stored.PrivateKey, nil
}

func (s *FileKeyStore) Save(privateKey string) error {
	if s == nil || s.path == "" {
		return nil
	}
	raw, err := json.MarshalIndent(keyLedger{PrivateKey: privateKey}, "", "  ")
	if err != nil {
		return err
	}
	return db.AtomicWritePrivateFile(s.path, raw)
}
