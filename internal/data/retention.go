package data

import (
	"fmt"
	"os"
	"time"

	dataapi "ardents/internal/data/api"
	retentionpkg "ardents/internal/data/retention"
)

func (s *Service) RetainRelayBlob(blob Blob, payload []byte, expiresAt time.Time) (Blob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	if len(payload) == 0 {
		return Blob{}, fmt.Errorf("relay retention requires payload")
	}
	var err error
	expiresAt, err = retentionpkg.ResolveRelayExpiry(now, expiresAt, s.cfg.DefaultRelayRetentionTTL)
	if err != nil {
		return Blob{}, err
	}
	blob, err = retentionpkg.PrepareRelayBlob(blob, payload, expiresAt, retentionAuthorizer(s.retention))
	if err != nil {
		return Blob{}, err
	}
	if err := s.ensureRelayRetentionBudgetLocked(blob.ID, payload); err != nil {
		return Blob{}, err
	}
	if blob.CreatedAt.IsZero() {
		blob.CreatedAt = now
	}
	if err := s.writePayloadLocked(blob.ID, payload); err != nil {
		return Blob{}, err
	}
	s.blobs.Put(blob)
	s.state = "ready"
	return blob, s.saveLocked()
}

func (s *Service) RetainBlob(id string, expiresAt time.Time) (Blob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	blob, err := retentionpkg.RetainBlob(&s.blobs, id, expiresAt, now, s.cfg.DefaultLocalRetentionTTL, s.hasLocalPayloadLocked, retentionAuthorizer(s.retention))
	if err != nil {
		return Blob{}, err
	}
	s.blobs.Put(blob)
	s.state = "ready"
	return blob, s.saveLocked()
}

func (s *Service) PinBlob(id string) (Blob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	blob, err := retentionpkg.PinBlob(&s.blobs, id, s.hasLocalPayloadLocked)
	if err != nil {
		return Blob{}, err
	}
	s.blobs.Put(blob)
	s.state = "ready"
	return blob, s.saveLocked()
}

func (s *Service) DropBlob(id string) (Blob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	blob, err := retentionpkg.DropBlob(&s.blobs, id, func(id string) error {
		err := os.Remove(s.payloadPath(id))
		if os.IsNotExist(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return Blob{}, err
	}
	s.blobs.Put(blob)
	s.state = "ready"
	return blob, s.saveLocked()
}

func (s *Service) PruneExpired(now time.Time) ([]Blob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now = now.UTC()
	prunedIDs, changed, err := retentionpkg.PruneExpired(&s.blobs, now, func(id string) error {
		err := os.Remove(s.payloadPath(id))
		if os.IsNotExist(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	if !changed {
		return nil, nil
	}
	pruned := make([]Blob, 0, len(prunedIDs))
	for _, id := range prunedIDs {
		pruned = append(pruned, s.blobs.Items[id])
	}
	s.state = "ready"
	return append([]Blob(nil), pruned...), s.saveLocked()
}

func (s *Service) ensureRelayRetentionBudgetLocked(id string, payload []byte) error {
	if s.cfg.MaxRelayRetentionBytes <= 0 {
		return nil
	}
	if s.relayBytesWithoutLocked(id)+int64(len(payload)) <= s.cfg.MaxRelayRetentionBytes {
		return nil
	}
	return fmt.Errorf("relay retention byte limit exceeded")
}

func (s *Service) hasLocalPayloadLocked(id string) bool {
	_, err := os.Stat(s.payloadPath(id))
	return err == nil
}

func (s *Service) relayBytesLocked() int64 {
	return retentionpkg.RelayBytes(&s.blobs, "", s.localPayloadInfoLocked)
}

func retentionAuthorizer(fn RetentionAuthorizer) retentionpkg.Authorizer {
	if fn == nil {
		return nil
	}
	return func(blob dataapi.BlobSnapshot, relay bool, expiresAt time.Time) error {
		return fn(blob, relay, expiresAt)
	}
}
