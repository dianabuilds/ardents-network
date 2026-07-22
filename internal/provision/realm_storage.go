package provision

import (
	"ardents/internal/storage"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type protectedNodeStorage struct {
	capabilityStore string
	storeKey        []byte
	storeKeyPath    string
	replayKeyPath   string
	discoveryReplay string
	dataReplay      string
	recordPath      string
}

func prepareNodeStorage(options NodeOptions) (protectedNodeStorage, error) {
	nodeStorage := protectedNodeStorage{
		capabilityStore: filepath.Join(options.DataDir, "capabilities.db"),
		storeKeyPath:    filepath.Join(options.SecretDir, "capability-store.key"),
		replayKeyPath:   filepath.Join(options.SecretDir, "replay.key"),
		discoveryReplay: filepath.Join(options.DataDir, "discovery-replay.db"),
		dataReplay:      filepath.Join(options.DataDir, "data-replay.db"),
		recordPath:      filepath.Join(options.SecretDir, "local-realm-node.json"),
	}
	key, err := loadOrCreateKey(nodeStorage.storeKeyPath, nodeStorage.capabilityStore)
	if err != nil {
		return protectedNodeStorage{}, err
	}
	if _, err := loadOrCreateKey(nodeStorage.replayKeyPath, nodeStorage.discoveryReplay); err != nil {
		return protectedNodeStorage{}, err
	}
	nodeStorage.storeKey = key
	return nodeStorage, nil
}

func ensurePrivateDir(path string) error {
	return storage.EnsurePrivateDir(path)
}

func readPrivateJSON(path string, target any) error {
	raw, err := readPrivate(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode protected state")
	}
	return nil
}

func readPrivate(path string) ([]byte, error) {
	raw, found, err := storage.ReadPrivateFile(path)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, os.ErrNotExist
	}
	return raw, nil
}

func writePrivateJSON(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return storage.AtomicWritePrivateFile(path, raw)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
