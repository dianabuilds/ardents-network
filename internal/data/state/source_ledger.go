package state

import model "ardents/internal/data/model"

type SourceLedger struct {
	ByBlob map[string][]model.BlobSourceRecord
}

func NewSourceLedger() SourceLedger {
	return SourceLedger{ByBlob: map[string][]model.BlobSourceRecord{}}
}

func (s *SourceLedger) Load(items map[string][]model.BlobSourceRecord) {
	if items == nil {
		s.ByBlob = map[string][]model.BlobSourceRecord{}
		return
	}
	s.ByBlob = items
}

func (s *SourceLedger) Snapshot() map[string][]model.BlobSourceRecord {
	return s.ByBlob
}

func (s *SourceLedger) List(blobID string) []model.BlobSourceRecord {
	return s.ByBlob[blobID]
}

func (s *SourceLedger) Replace(blobID string, items []model.BlobSourceRecord) {
	s.ByBlob[blobID] = items
}
