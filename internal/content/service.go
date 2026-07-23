package content

import (
	"ardents/internal/content/catalog"
	datapayload "ardents/internal/content/payload"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Service struct {
	mu          sync.Mutex
	path        string
	dir         string
	cfg         Config
	state       string
	localNodeID string
	retention   RetentionAuthorizer
	objects     catalog.ObjectStore
	blobs       catalog.BlobStore
	blobOwners  catalog.BlobOwnerStore
	sources     catalog.SourceLedger
	manifests   catalog.ManifestStore
}

func New(path string) *Service {
	return NewWithConfig(path, Config{})
}

func NewWithConfig(path string, cfg Config) *Service {
	dir := filepath.Dir(path)
	service := &Service{
		path:       path,
		dir:        dir,
		cfg:        cfg,
		state:      "new",
		objects:    catalog.NewObjectStore(),
		blobs:      catalog.NewBlobStore(),
		blobOwners: catalog.NewBlobOwnerStore(),
		sources:    catalog.NewSourceLedger(),
		manifests:  catalog.NewManifestStore(),
	}
	return service
}

func NewInDir(dir string) *Service {
	return New(contentPath(dir))
}

func NewInDirWithConfig(dir string, cfg Config) *Service {
	return NewWithConfig(contentPath(dir), cfg)
}

func (s *Service) now() time.Time {
	if s.cfg.Now != nil {
		return s.cfg.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) SetRetentionAuthorizer(fn RetentionAuthorizer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retention = fn
}

func (s *Service) SetLocalNodeID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.localNodeID = id
}

func (s *Service) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		s.state = "ready"
		return nil
	}
	var data persistedContent
	found, err := loadContent(s.path, &data)
	if err != nil {
		return err
	}
	if !found {
		data.Version = contentSchemaVersion
		data.BlobOwnership.Version = blobOwnershipVersion
	}
	if data.Version != contentSchemaVersion {
		return fmt.Errorf("unsupported content schema version")
	}
	if data.BlobOwnership.Version != blobOwnershipVersion {
		return fmt.Errorf("unsupported blob ownership version")
	}
	normalizeSnapshot(&data)
	for _, object := range data.Objects {
		if object.Owner.String() == "" {
			return fmt.Errorf("persisted object owner is invalid")
		}
	}
	for _, manifest := range data.Manifests {
		if manifest.Owner.String() == "" {
			return fmt.Errorf("persisted manifest owner is invalid")
		}
	}
	s.objects.Load(data.Objects)
	s.blobs.Load(data.Blobs)
	if err := s.blobOwners.Load(data.BlobOwnership.Bindings, data.Blobs); err != nil {
		return err
	}
	s.sources.Load(data.Sources)
	s.manifests.Load(data.Manifests)
	if err := s.removeUntrackedPayloadsLocked(); err != nil {
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
	blobs := s.blobs.Snapshot()
	known := make(map[string]bool, len(blobs))
	for id := range blobs {
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
	for _, manifest := range s.manifests.Snapshot() {
		for _, ref := range manifest.Refs {
			if ref.Kind == "blob" {
				retentionByBlob[ref.ID] = manifest.Retention
			}
		}
	}
	changed := false
	for id, blob := range s.blobs.Snapshot() {
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

func normalizeSnapshot(data *persistedContent) {
	if data.Objects == nil {
		data.Objects = map[string]catalog.Object{}
	}
	if data.Blobs == nil {
		data.Blobs = map[string]Blob{}
	}
	if data.Sources == nil {
		data.Sources = map[string][]BlobSourceRecord{}
	}
	if data.Manifests == nil {
		data.Manifests = map[string]catalog.Manifest{}
	}
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
	return saveContent(s.path, persistedContent{
		Version:   contentSchemaVersion,
		Objects:   s.objects.Snapshot(),
		Blobs:     s.blobs.Snapshot(),
		Sources:   s.sources.Snapshot(),
		Manifests: s.manifests.Snapshot(),
		BlobOwnership: persistedBlobOwnership{
			Version: blobOwnershipVersion, Bindings: s.blobOwners.Snapshot(),
		},
	})
}

func (s *Service) reconcileLoadedStateLocked(now time.Time) error {
	updated, changed, err := ReconcileLoadedBlobs(
		s.blobs.Snapshot(),
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

type RemovePayload func(id string) error

func ReconcileLoadedBlobs(blobs map[string]catalog.Blob, now time.Time, hasLocalPayload LocalPayloadPresence, removePayload RemovePayload) (map[string]catalog.Blob, bool, error) {
	updated := cloneBlobs(blobs)
	changed, err := pruneExpired(updated, now.UTC(), removePayload)
	if err != nil {
		return nil, false, err
	}
	if reconcileMissingPayload(updated, hasLocalPayload) {
		changed = true
	}
	if !changed {
		return blobs, false, nil
	}
	return updated, true, nil
}

func pruneExpired(blobs map[string]catalog.Blob, now time.Time, removePayload RemovePayload) (bool, error) {
	changed := false
	for id, blob := range blobs {
		if blob.State != "retained-temporary" || blob.ExpiresAt.IsZero() || blob.ExpiresAt.After(now) {
			continue
		}
		if err := removePayload(id); err != nil {
			return false, err
		}
		blob.State = "expired"
		blobs[id] = blob
		changed = true
	}
	return changed, nil
}

func reconcileMissingPayload(blobs map[string]catalog.Blob, hasLocalPayload LocalPayloadPresence) bool {
	changed := false
	for id, blob := range blobs {
		if !datapayload.StateRequiresLocalPayload(blob.State) || hasLocalPayload(id) {
			continue
		}
		blob.State = "deleted"
		blob.ExpiresAt = time.Time{}
		blobs[id] = blob
		changed = true
	}
	return changed
}

func cloneBlobs(items map[string]catalog.Blob) map[string]catalog.Blob {
	cloned := make(map[string]catalog.Blob, len(items))
	maps.Copy(cloned, items)
	return cloned
}

func (s *Service) State() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Service) ObjectState() string {
	state, _ := s.ObjectPartState()
	return state
}

func (s *Service) BlobState() string {
	state, _ := s.BlobPartState()
	return state
}

func (s *Service) ManifestState() string {
	state, _ := s.ObjectPartState()
	return state
}

func (s *Service) ObjectPartState() (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return ObjectPartState(s.state, s.objects.Snapshot(), s.manifests.Snapshot(), s.blobs.Snapshot())
}

func (s *Service) BlobPartState() (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return BlobPartState(s.state, s.blobs.Snapshot(), s.hasLocalPayloadLocked)
}

func (s *Service) Inventory() Inventory {
	s.mu.Lock()
	defer s.mu.Unlock()
	return ProjectInventory(s.objects.Count(), s.manifests.Count(), s.blobs.Snapshot(), s.localPayloadInfoLocked)
}

func (s *Service) ObjectPart() PartSnapshot {
	state, reason := s.ObjectPartState()
	return PartSnapshot{State: state, Reason: reason}
}

func (s *Service) BlobPart() PartSnapshot {
	state, reason := s.BlobPartState()
	return PartSnapshot{State: state, Reason: reason}
}

func (s *Service) localPayloadInfoLocked(id string) (bool, int64) {
	info, err := os.Stat(s.payloadPath(id))
	if err != nil {
		return false, 0
	}
	return true, info.Size()
}

type LocalPayloadPresence func(id string) bool

func ObjectPartState(nodeState string, objects map[string]catalog.Object, manifests map[string]catalog.Manifest, blobs map[string]catalog.Blob) (string, string) {
	if nodeState != "ready" {
		return nodeState, ""
	}
	missing, sample := countMissingMetadataBlobRefs(objects, manifests, blobs)
	if missing == 0 {
		return "ready", ""
	}
	return "degraded", formatBrokenRefReason(sample, missing)
}

func BlobPartState(nodeState string, blobs map[string]catalog.Blob, hasLocalPayload LocalPayloadPresence) (string, string) {
	if nodeState != "ready" {
		return nodeState, ""
	}
	missing, sample := countMissingLocalPayloads(blobs, hasLocalPayload)
	if missing == 0 {
		return "ready", ""
	}
	return "degraded", formatMissingPayloadReason(sample, missing)
}

func countMissingMetadataBlobRefs(objects map[string]catalog.Object, manifests map[string]catalog.Manifest, blobs map[string]catalog.Blob) (int, string) {
	count := 0
	sample := ""
	for _, id := range sortedObjectIDs(objects) {
		object := objects[id]
		for _, ref := range object.BlobRefs {
			if ref.Kind != "blob" || ref.ID == "" {
				continue
			}
			if _, ok := blobs[ref.ID]; ok {
				continue
			}
			count++
			if sample == "" {
				sample = fmt.Sprintf("object %q references missing blob %q", id, ref.ID)
			}
		}
	}
	for _, id := range sortedManifestIDs(manifests) {
		manifest := manifests[id]
		for _, ref := range manifest.Refs {
			if ref.Kind != "blob" || ref.ID == "" {
				continue
			}
			if _, ok := blobs[ref.ID]; ok {
				continue
			}
			count++
			if sample == "" {
				sample = fmt.Sprintf("manifest %q references missing blob %q", id, ref.ID)
			}
		}
	}
	return count, sample
}

func countMissingLocalPayloads(blobs map[string]catalog.Blob, hasLocalPayload LocalPayloadPresence) (int, string) {
	count := 0
	sample := ""
	for _, id := range sortedBlobIDs(blobs) {
		blob := blobs[id]
		if !datapayload.StateRequiresLocalPayload(blob.State) || hasLocalPayload(id) {
			continue
		}
		count++
		if sample == "" {
			sample = fmt.Sprintf("blob %q is %q without local payload", id, blob.State)
		}
	}
	return count, sample
}

func sortedObjectIDs(items map[string]catalog.Object) []string {
	ids := make([]string, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func sortedBlobIDs(items map[string]catalog.Blob) []string {
	ids := make([]string, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func sortedManifestIDs(items map[string]catalog.Manifest) []string {
	ids := make([]string, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func formatBrokenRefReason(sample string, count int) string {
	if count <= 1 {
		return sample
	}
	return fmt.Sprintf("%s (%d broken blob refs total)", sample, count)
}

func formatMissingPayloadReason(sample string, count int) string {
	if count <= 1 {
		return sample
	}
	return fmt.Sprintf("%s (%d local payloads missing total)", sample, count)
}
