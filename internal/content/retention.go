package content

import (
	model "ardents/internal/content/catalog"
	datapayload "ardents/internal/content/payload"
	"ardents/internal/identity/principal"
	"errors"
	"fmt"
	"os"
	"time"
)

func (s *Service) RetainRelayBlob(blob Blob, payload []byte, expiresAt time.Time) (Blob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	if len(payload) == 0 {
		return Blob{}, fmt.Errorf("relay retention requires payload")
	}
	var err error
	expiresAt, err = ResolveRelayExpiry(now, expiresAt, s.cfg.DefaultRelayRetentionTTL)
	if err != nil {
		return Blob{}, err
	}
	blob, err = PrepareRelayBlob(blob, payload, expiresAt, s.retention)
	if err != nil {
		return Blob{}, err
	}
	reference := blob.Reference.String()
	if err := s.ensureRelayRetentionBudgetLocked(reference, payload); err != nil {
		return Blob{}, err
	}
	if blob.CreatedAt.IsZero() {
		blob.CreatedAt = now
	}
	previous, previousState := s.blobs.Snapshot(), s.state
	if err := s.writePayloadLocked(reference, payload); err != nil {
		return Blob{}, err
	}
	s.blobs.Put(blob)
	s.state = "ready"
	if err := s.saveLocked(); err != nil {
		return Blob{}, errors.Join(err, s.rollbackUncommittedPayloadLocked(previous, previousState, reference))
	}
	return blob, nil
}

func (s *Service) RetainBlob(id string, expiresAt time.Time) (Blob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	previous, previousState := s.blobs.Snapshot(), s.state
	now := time.Now().UTC()
	blob, err := RetainBlob(&s.blobs, id, expiresAt, now, s.cfg.DefaultLocalRetentionTTL, s.hasLocalPayloadLocked, s.retention)
	if err != nil {
		return Blob{}, err
	}
	s.blobs.Put(blob)
	s.state = "ready"
	if err := s.saveLocked(); err != nil {
		s.blobs.Load(previous)
		s.state = previousState
		return Blob{}, err
	}
	return blob, nil
}

func (s *Service) ExtendRelayRetention(id string, expiresAt time.Time) (Blob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	blob, ok := s.blobs.Get(id)
	if !ok || !s.hasLocalPayloadLocked(id) {
		return Blob{}, fmt.Errorf("relay-retained blob is not locally available")
	}
	expiresAt = expiresAt.UTC()
	if expiresAt.IsZero() || (!blob.ExpiresAt.IsZero() && expiresAt.Before(blob.ExpiresAt)) {
		return Blob{}, fmt.Errorf("relay retention expiry is invalid")
	}
	blob.ExpiresAt = expiresAt
	s.blobs.Put(blob)
	return blob, s.saveLocked()
}

func (s *Service) PinBlob(id string) (Blob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	previous, previousState := s.blobs.Snapshot(), s.state
	blob, err := PinBlob(&s.blobs, id, s.hasLocalPayloadLocked)
	if err != nil {
		return Blob{}, err
	}
	s.blobs.Put(blob)
	s.state = "ready"
	if err := s.saveLocked(); err != nil {
		s.blobs.Load(previous)
		s.state = previousState
		return Blob{}, err
	}
	return blob, nil
}

func (s *Service) DropBlob(id string) (Blob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	previous, previousState := s.blobs.Snapshot(), s.state
	removals := payloadRemovalBatch{service: s}
	blob, err := DropBlob(&s.blobs, id, removals.Stage)
	if err != nil {
		s.blobs.Load(previous)
		s.state = previousState
		return Blob{}, errors.Join(err, removals.Rollback())
	}
	s.blobs.Put(blob)
	s.state = "ready"
	if err := s.saveLocked(); err != nil {
		s.blobs.Load(previous)
		s.state = previousState
		return Blob{}, errors.Join(err, removals.Rollback())
	}
	if err := removals.Commit(); err != nil {
		return blob, err
	}
	return blob, nil
}

// DropBlobForOwner removes exactly one Principal binding. The shared payload is
// reclaimed only when no other owner, Object/Manifest reference, or independent
// retention fact still needs it.
func (s *Service) DropBlobForOwner(owner principal.ID, id string) (Blob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reference, err := model.ParseContentReference(id)
	if err != nil || owner.String() == "" || !s.blobOwners.Has(owner, reference) {
		return Blob{}, ErrBlobNotFound
	}
	blob, ok := s.blobs.Get(id)
	if !ok {
		return Blob{}, ErrBlobNotFound
	}
	previousBlobs, previousOwners, previousState := s.blobs.Snapshot(), s.blobOwners.Snapshot(), s.state
	s.blobOwners.Delete(owner, reference)
	shouldKeep := s.blobOwners.CountReference(reference) > 0 || s.blobReferencedLocked(id) || independentlyRetained(blob)
	removals := payloadRemovalBatch{service: s}
	if !shouldKeep {
		if err := removals.Stage(id); err != nil {
			_ = s.restoreOwnedBlobSnapshotLocked(previousBlobs, previousOwners, previousState)
			return Blob{}, err
		}
		blob.State = "deleted"
		blob.ExpiresAt = time.Time{}
		s.blobs.Put(blob)
	}
	s.state = "ready"
	if err := s.saveLocked(); err != nil {
		restoreErr := s.restoreOwnedBlobSnapshotLocked(previousBlobs, previousOwners, previousState)
		return Blob{}, errors.Join(err, restoreErr, removals.Rollback())
	}
	if err := removals.Commit(); err != nil {
		return blob, err
	}
	return blob, nil
}

func (s *Service) blobReferencedLocked(id string) bool {
	for _, object := range s.objects.Snapshot() {
		for _, ref := range object.BlobRefs {
			if ref.Kind == "blob" && ref.ID == id {
				return true
			}
		}
	}
	for _, manifest := range s.manifests.Snapshot() {
		for _, ref := range manifest.Refs {
			if ref.Kind == "blob" && ref.ID == id {
				return true
			}
		}
	}
	return false
}

func independentlyRetained(blob Blob) bool {
	return blob.State == "pinned" || blob.State == "retained-temporary" || blob.Retention == "durable" ||
		blob.Retention == "relay-temporary" || blob.Retention == "staging"
}

func (s *Service) PruneExpired(now time.Time) ([]Blob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	previous, previousState := s.blobs.Snapshot(), s.state
	removals := payloadRemovalBatch{service: s}
	now = now.UTC()
	prunedIDs, changed, err := PruneExpired(&s.blobs, now, removals.Stage)
	if err != nil {
		s.blobs.Load(previous)
		s.state = previousState
		return nil, errors.Join(err, removals.Rollback())
	}
	if !changed {
		return nil, nil
	}
	pruned := make([]Blob, 0, len(prunedIDs))
	for _, id := range prunedIDs {
		blob, _ := s.blobs.Get(id)
		pruned = append(pruned, blob)
	}
	s.state = "ready"
	if err := s.saveLocked(); err != nil {
		s.blobs.Load(previous)
		s.state = previousState
		return nil, errors.Join(err, removals.Rollback())
	}
	if err := removals.Commit(); err != nil {
		return append([]Blob(nil), pruned...), err
	}
	return append([]Blob(nil), pruned...), nil
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
	return RelayBytes(&s.blobs, "", s.localPayloadInfoLocked)
}

type retentionPayloadInfo func(id string) (bool, int64)

func ResolveRelayExpiry(now, expiresAt time.Time, defaultTTL time.Duration) (time.Time, error) {
	if !expiresAt.IsZero() {
		return expiresAt.UTC(), nil
	}
	if defaultTTL <= 0 {
		return time.Time{}, fmt.Errorf("relay retention expiry is required")
	}
	return now.Add(defaultTTL), nil
}

func PrepareRelayBlob(blob model.Blob, payload []byte, expiresAt time.Time, authorize RetentionAuthorizer) (model.Blob, error) {
	blob = NormalizeBlob(blob)
	if !blob.Encrypted {
		return model.Blob{}, fmt.Errorf("relay retention requires encrypted blob")
	}
	if authorize != nil {
		if err := authorize(BlobPolicySnapshot(blob), true, expiresAt.UTC()); err != nil {
			return model.Blob{}, err
		}
	}
	hash, blobCID, err := datapayload.DeriveIdentity(payload)
	if err != nil {
		return model.Blob{}, err
	}
	if err := datapayload.ApplyDerivedIdentity(&blob, hash, blobCID); err != nil {
		return model.Blob{}, err
	}
	blob.Size = int64(len(payload))
	blob.State = "retained-temporary"
	blob.Retention = "relay-temporary"
	blob.ExpiresAt = expiresAt.UTC()
	return blob, nil
}

func RetainBlob(
	blobs *model.BlobStore,
	id string,
	expiresAt time.Time,
	now time.Time,
	defaultTTL time.Duration,
	hasLocalPayload func(string) bool,
	authorize RetentionAuthorizer,
) (model.Blob, error) {
	blob, ok := blobs.Get(id)
	if !ok {
		return model.Blob{}, fmt.Errorf("blob not found")
	}
	if !hasLocalPayload(id) {
		return model.Blob{}, fmt.Errorf("blob payload is not locally available")
	}
	if expiresAt.IsZero() {
		if defaultTTL <= 0 {
			return model.Blob{}, fmt.Errorf("retention expiry is required")
		}
		expiresAt = now.Add(defaultTTL)
	}
	if authorize != nil {
		if err := authorize(BlobPolicySnapshot(blob), false, expiresAt.UTC()); err != nil {
			return model.Blob{}, err
		}
	}
	blob.State = "retained-temporary"
	blob.Retention = "temporary"
	blob.ExpiresAt = expiresAt.UTC()
	blobs.Put(blob)
	return blob, nil
}

func PinBlob(blobs *model.BlobStore, id string, hasLocalPayload func(string) bool) (model.Blob, error) {
	blob, ok := blobs.Get(id)
	if !ok {
		return model.Blob{}, fmt.Errorf("blob not found")
	}
	if !hasLocalPayload(id) {
		return model.Blob{}, fmt.Errorf("blob payload is not locally available")
	}
	blob.State = "pinned"
	blob.Retention = "pinned"
	blob.ExpiresAt = time.Time{}
	blobs.Put(blob)
	return blob, nil
}

func DropBlob(blobs *model.BlobStore, id string, removePayload func(string) error) (model.Blob, error) {
	blob, ok := blobs.Get(id)
	if !ok {
		return model.Blob{}, fmt.Errorf("blob not found")
	}
	if err := removePayload(id); err != nil {
		return model.Blob{}, err
	}
	blob.State = "deleted"
	blob.ExpiresAt = time.Time{}
	blobs.Put(blob)
	return blob, nil
}

func PruneExpired(blobs *model.BlobStore, now time.Time, removePayload func(string) error) ([]string, bool, error) {
	var (
		changed   bool
		prunedIDs []string
	)
	for id, blob := range blobs.Items {
		if blob.State != "retained-temporary" || blob.ExpiresAt.IsZero() || blob.ExpiresAt.After(now) {
			continue
		}
		if err := removePayload(id); err != nil {
			return nil, false, err
		}
		blob.State = "expired"
		blobs.Put(blob)
		prunedIDs = append(prunedIDs, id)
		changed = true
	}
	return prunedIDs, changed, nil
}

func RelayBytes(blobs *model.BlobStore, excludeID string, localPayloadInfo retentionPayloadInfo) int64 {
	var total int64
	for id, blob := range blobs.Items {
		if id == excludeID || blob.Retention != "relay-temporary" || blob.State == "deleted" || blob.State == "expired" {
			continue
		}
		total += relayBlobBytes(id, blob, localPayloadInfo)
	}
	return total
}

func NormalizeBlob(blob model.Blob) model.Blob {
	if blob.Retention == "" {
		blob.Retention = "owner"
	}
	return blob
}

func BlobPolicySnapshot(blob model.Blob) BlobPolicyView {
	return BlobPolicyView{
		Reference: blob.Reference,
		MediaType: blob.MediaType,
		Size:      blob.Size,
		Hash:      blob.Hash,
		Cipher:    blob.Cipher,
		KeyID:     blob.KeyID,
		State:     blob.State,
		Retention: blob.Retention,
		Encrypted: blob.Encrypted,
		ExpiresAt: blob.ExpiresAt,
		CreatedAt: blob.CreatedAt,
	}
}

func relayBlobBytes(id string, blob model.Blob, localPayloadInfo retentionPayloadInfo) int64 {
	if blob.Size != 0 {
		return blob.Size
	}
	present, size := localPayloadInfo(id)
	if !present {
		return 0
	}
	return size
}
