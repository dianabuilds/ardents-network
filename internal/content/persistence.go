package content

import (
	"ardents/internal/content/catalog"
	"ardents/internal/storage"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type persistedContent struct {
	Version       uint32                                `json:"version"`
	Objects       map[string]catalog.Object             `json:"objects"`
	Blobs         map[string]catalog.Blob               `json:"blobs"`
	Sources       map[string][]catalog.BlobSourceRecord `json:"sources"`
	Manifests     map[string]catalog.Manifest           `json:"manifests"`
	BlobOwnership persistedBlobOwnership                `json:"blob_ownership"`
}

const contentSchemaVersion = 3
const legacyContentSchemaVersion = 2
const blobOwnershipVersion = 1

type persistedBlobOwnership struct {
	Version  uint32                     `json:"version"`
	Bindings []catalog.BlobOwnerBinding `json:"bindings"`
}

func contentPath(dir string) string { return storage.PathInDir(dir) }

func loadContent(path string, out *persistedContent) (bool, error) {
	return storage.LoadJSONStrict(path, "data", "snapshot", out)
}

func saveContent(path string, snapshot persistedContent) error {
	return storage.SaveJSON(path, "data", "snapshot", snapshot)
}

type stagedPayload struct {
	source string
	staged string
}

type payloadRemovalBatch struct {
	service *Service
	items   []stagedPayload
}

func (s *Service) restoreBlobSnapshotLocked(previous map[string]Blob, state string) {
	s.blobs.Load(previous)
	s.state = state
}

func (s *Service) rollbackUncommittedPayloadLocked(previous map[string]Blob, state, id string) error {
	_, existed := previous[id]
	s.restoreBlobSnapshotLocked(previous, state)
	if existed {
		return nil
	}
	if err := os.Remove(s.payloadPath(id)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove uncommitted payload")
	}
	return nil
}

func (b *payloadRemovalBatch) Stage(id string) error {
	source := b.service.payloadPath(id)
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect payload for removal")
	}
	staged, err := privateRemovalPath(filepath.Dir(source))
	if err != nil {
		return err
	}
	if err := os.Rename(source, staged); err != nil {
		return fmt.Errorf("stage payload removal")
	}
	b.items = append(b.items, stagedPayload{source: source, staged: staged})
	return nil
}

func (b *payloadRemovalBatch) Rollback() error {
	var failures []error
	for index := len(b.items) - 1; index >= 0; index-- {
		item := b.items[index]
		if err := os.Rename(item.staged, item.source); err != nil && !os.IsNotExist(err) {
			failures = append(failures, fmt.Errorf("restore staged payload"))
		}
	}
	return errors.Join(failures...)
}

func (b *payloadRemovalBatch) Commit() error {
	var failures []error
	for _, item := range b.items {
		if err := os.Remove(item.staged); err != nil && !os.IsNotExist(err) {
			failures = append(failures, fmt.Errorf("remove staged payload"))
		}
	}
	return errors.Join(failures...)
}

func privateRemovalPath(dir string) (string, error) {
	for range 4 {
		var token [16]byte
		if _, err := rand.Read(token[:]); err != nil {
			return "", fmt.Errorf("allocate private payload removal path")
		}
		path := filepath.Join(dir, ".ardents-private-delete-"+hex.EncodeToString(token[:]))
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			return path, nil
		}
	}
	return "", fmt.Errorf("cannot allocate private payload removal path")
}
