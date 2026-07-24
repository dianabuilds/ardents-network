package catalog

import (
	"ardents/internal/identity/principal"
	"encoding/base64"
	"fmt"
	"maps"
	"sort"
)

type BlobStore struct {
	Items map[string]Blob
}

func NewBlobStore() BlobStore { return BlobStore{Items: map[string]Blob{}} }
func (s *BlobStore) Load(items map[string]Blob) {
	if items == nil {
		items = map[string]Blob{}
	}
	s.Items = items
}
func (s *BlobStore) Snapshot() map[string]Blob  { return maps.Clone(s.Items) }
func (s *BlobStore) Get(id string) (Blob, bool) { item, ok := s.Items[id]; return item, ok }
func (s *BlobStore) Put(item Blob)              { s.Items[item.Reference.String()] = item }
func (s *BlobStore) Delete(id string)           { delete(s.Items, id) }
func (s *BlobStore) Count() int                 { return len(s.Items) }

type blobOwnerKey struct {
	Owner     principal.ID
	Reference ContentReference
}

type BlobOwnerStore struct {
	items map[blobOwnerKey]BlobOwnerBinding
}

func NewBlobOwnerStore() BlobOwnerStore {
	return BlobOwnerStore{items: map[blobOwnerKey]BlobOwnerBinding{}}
}

func (s *BlobOwnerStore) Load(items []BlobOwnerBinding, blobs map[string]Blob) error {
	loaded := make(map[blobOwnerKey]BlobOwnerBinding, len(items))
	for _, item := range items {
		if item.Owner.String() == "" || item.Reference.String() == "" || item.CreatedAt.IsZero() {
			return fmt.Errorf("blob owner binding is invalid")
		}
		if _, ok := blobs[item.Reference.String()]; !ok {
			return fmt.Errorf("blob owner binding references unknown content")
		}
		key := blobOwnerKey{Owner: item.Owner, Reference: item.Reference}
		if _, duplicate := loaded[key]; duplicate {
			return fmt.Errorf("duplicate blob owner binding")
		}
		loaded[key] = item
	}
	s.items = loaded
	return nil
}

func (s *BlobOwnerStore) Snapshot() []BlobOwnerBinding {
	items := make([]BlobOwnerBinding, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		leftOwner, rightOwner := items[i].Owner.String(), items[j].Owner.String()
		if leftOwner != rightOwner {
			return leftOwner < rightOwner
		}
		return items[i].Reference.String() < items[j].Reference.String()
	})
	return items
}

func (s *BlobOwnerStore) Has(owner principal.ID, reference ContentReference) bool {
	_, ok := s.items[blobOwnerKey{Owner: owner, Reference: reference}]
	return ok
}

func (s *BlobOwnerStore) Put(item BlobOwnerBinding) {
	s.items[blobOwnerKey{Owner: item.Owner, Reference: item.Reference}] = item
}

func (s *BlobOwnerStore) Delete(owner principal.ID, reference ContentReference) {
	delete(s.items, blobOwnerKey{Owner: owner, Reference: reference})
}

func (s *BlobOwnerStore) CountReference(reference ContentReference) int {
	count := 0
	for key := range s.items {
		if key.Reference == reference {
			count++
		}
	}
	return count
}

func (s *BlobOwnerStore) Count() int { return len(s.items) }

type ObjectStore struct {
	Items map[string]Object
}

func NewObjectStore() ObjectStore { return ObjectStore{Items: map[string]Object{}} }
func (s *ObjectStore) Load(items map[string]Object) {
	if items == nil {
		items = map[string]Object{}
	}
	s.Items = items
}
func (s *ObjectStore) Snapshot() map[string]Object { return cloneObjects(s.Items) }
func (s *ObjectStore) Get(owner principal.ID, id string) (Object, bool) {
	item, ok := s.Items[RecordStorageKey(owner, id)]
	return item, ok
}
func (s *ObjectStore) Put(item Object) error {
	if item.Owner.String() == "" || item.ID == "" {
		return fmt.Errorf("object owner-qualified identity is invalid")
	}
	s.Items[RecordStorageKey(item.Owner, item.ID)] = item
	return nil
}
func (s *ObjectStore) Count() int { return len(s.Items) }

type ManifestStore struct {
	Items map[string]Manifest
}

func NewManifestStore() ManifestStore { return ManifestStore{Items: map[string]Manifest{}} }
func (s *ManifestStore) Load(items map[string]Manifest) {
	if items == nil {
		items = map[string]Manifest{}
	}
	s.Items = items
}
func (s *ManifestStore) Snapshot() map[string]Manifest { return cloneManifests(s.Items) }
func (s *ManifestStore) Get(owner principal.ID, id string) (Manifest, bool) {
	item, ok := s.Items[RecordStorageKey(owner, id)]
	return item, ok
}
func (s *ManifestStore) Put(item Manifest) error {
	if item.Owner.String() == "" || item.ID == "" {
		return fmt.Errorf("manifest owner-qualified identity is invalid")
	}
	s.Items[RecordStorageKey(item.Owner, item.ID)] = item
	return nil
}
func (s *ManifestStore) Delete(owner principal.ID, id string) {
	delete(s.Items, RecordStorageKey(owner, id))
}
func (s *ManifestStore) Count() int { return len(s.Items) }

func RecordStorageKey(owner principal.ID, id string) string {
	encode := base64.RawURLEncoding.EncodeToString
	return encode([]byte(owner.String())) + "." + encode([]byte(id))
}

type SourceLedger struct {
	ByBlob map[string][]BlobSourceRecord
}

func NewSourceLedger() SourceLedger { return SourceLedger{ByBlob: map[string][]BlobSourceRecord{}} }
func (s *SourceLedger) Load(items map[string][]BlobSourceRecord) {
	if items == nil {
		items = map[string][]BlobSourceRecord{}
	}
	s.ByBlob = items
}
func (s *SourceLedger) Snapshot() map[string][]BlobSourceRecord {
	items := make(map[string][]BlobSourceRecord, len(s.ByBlob))
	for id, records := range s.ByBlob {
		items[id] = append([]BlobSourceRecord(nil), records...)
	}
	return items
}
func (s *SourceLedger) List(id string) []BlobSourceRecord           { return s.ByBlob[id] }
func (s *SourceLedger) Replace(id string, items []BlobSourceRecord) { s.ByBlob[id] = items }
