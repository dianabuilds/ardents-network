package catalog

import "maps"

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
func (s *BlobStore) Put(item Blob)              { s.Items[item.ID] = item }
func (s *BlobStore) Delete(id string)           { delete(s.Items, id) }
func (s *BlobStore) Count() int                 { return len(s.Items) }

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
func (s *ObjectStore) Snapshot() map[string]Object  { return cloneObjects(s.Items) }
func (s *ObjectStore) Get(id string) (Object, bool) { item, ok := s.Items[id]; return item, ok }
func (s *ObjectStore) Put(item Object)              { s.Items[item.ID] = item }
func (s *ObjectStore) Count() int                   { return len(s.Items) }

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
func (s *ManifestStore) Snapshot() map[string]Manifest  { return cloneManifests(s.Items) }
func (s *ManifestStore) Get(id string) (Manifest, bool) { item, ok := s.Items[id]; return item, ok }
func (s *ManifestStore) Put(item Manifest)              { s.Items[item.ID] = item }
func (s *ManifestStore) Delete(id string)               { delete(s.Items, id) }
func (s *ManifestStore) Count() int                     { return len(s.Items) }

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
