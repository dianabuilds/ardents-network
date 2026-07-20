package data

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	availabilitypkg "ardents/internal/data/availability"
	observedpkg "ardents/internal/data/observed"
	statepkg "ardents/internal/data/state"
)

func (s *Service) SetRetentionAuthorizer(fn RetentionAuthorizer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retention = fn
}

func (s *Service) SetLocalNodeID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.localNodeID = id
	s.placement.SetNodeID(id)
}

func (s *Service) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		s.state = "ready"
		return nil
	}
	var data statepkg.Snapshot
	_, err := statepkg.LoadSnapshot(s.path, &data)
	if err != nil {
		return err
	}
	normalizeSnapshot(&data)
	s.objects.Load(data.Objects)
	s.blobs.Load(data.Blobs)
	s.sources.Load(data.Sources)
	s.transfers.Load(data.Transfers)
	s.manifests.Load(data.Manifests)
	s.availability = availabilitypkg.Normalize(data.Availability)
	if err := s.removeUntrackedPayloadsLocked(); err != nil {
		return err
	}
	if err := s.placement.Restore(data.Placement); err != nil {
		return err
	}
	if err := s.reconcileChunkStagingLocked(); err != nil {
		return err
	}
	if err := s.reconcileLoadedStateLocked(time.Now().UTC()); err != nil {
		return err
	}
	s.state = "ready"
	return nil
}

func (s *Service) removeUntrackedPayloadsLocked() error {
	dir := filepath.Join(s.dir, "blobs")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	known := make(map[string]bool, len(s.blobs.Items))
	for id := range s.blobs.Items {
		known[filepath.Base(s.payloadPath(id))] = true
	}
	for _, entry := range entries {
		name := entry.Name()
		if known[name] || (!strings.HasSuffix(name, ".blob") && !strings.HasPrefix(name, ".ardents-private-")) {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("untracked payload entry must be a regular file")
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) reconcileChunkStagingLocked() error {
	retentionByBlob := make(map[string]string)
	for _, manifest := range s.manifests.Items {
		for _, ref := range manifest.Refs {
			if ref.Kind == "blob" {
				retentionByBlob[ref.ID] = manifest.Retention
			}
		}
	}
	changed := false
	for id, blob := range s.blobs.Items {
		if blob.Retention != "staging" {
			continue
		}
		if retention, referenced := retentionByBlob[id]; referenced {
			if retention == "" {
				retention = "durable"
			}
			blob.Retention = retention
			blob.State = "available-local"
			s.blobs.Put(blob)
			changed = true
			continue
		}
		if err := os.Remove(s.payloadPath(id)); err != nil && !os.IsNotExist(err) {
			return err
		}
		s.blobs.Delete(id)
		changed = true
	}
	if !changed {
		return nil
	}
	return s.saveLocked()
}

func normalizeSnapshot(data *statepkg.Snapshot) {
	if data.Objects == nil {
		data.Objects = map[string]Object{}
	}
	if data.Blobs == nil {
		data.Blobs = map[string]Blob{}
	}
	if data.Sources == nil {
		data.Sources = map[string][]BlobSourceRecord{}
	}
	if data.Transfers == nil {
		data.Transfers = map[string]TransferRecord{}
	}
	if data.Manifests == nil {
		data.Manifests = map[string]Manifest{}
	}
	data.Availability = availabilitypkg.Normalize(data.Availability)
}

func (s *Service) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Service) saveLocked() error {
	if s.path == "" {
		return nil
	}
	return statepkg.SaveSnapshot(s.path, statepkg.Snapshot{
		Objects:      s.objects.Snapshot(),
		Blobs:        s.blobs.Snapshot(),
		Sources:      s.sources.Snapshot(),
		Transfers:    s.transfers.Snapshot(),
		Manifests:    s.manifests.Snapshot(),
		Placement:    s.placement.Snapshot(),
		Availability: s.availability,
	})
}

func (s *Service) reconcileLoadedStateLocked(now time.Time) error {
	updated, changed, err := observedpkg.ReconcileLoadedBlobs(
		s.blobs.Items,
		now.UTC(),
		s.hasLocalPayloadLocked,
		func(id string) error {
			err := os.Remove(s.payloadPath(id))
			if os.IsNotExist(err) {
				return nil
			}
			return err
		},
	)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	s.blobs.Load(updated)
	return s.saveLocked()
}
