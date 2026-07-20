package state

import model "ardents/internal/data/model"

type TransferLedger struct {
	Items map[string]model.TransferRecord
}

func NewTransferLedger() TransferLedger {
	return TransferLedger{Items: map[string]model.TransferRecord{}}
}

func (s *TransferLedger) Load(items map[string]model.TransferRecord) {
	if items == nil {
		s.Items = map[string]model.TransferRecord{}
		return
	}
	s.Items = items
}

func (s *TransferLedger) Snapshot() map[string]model.TransferRecord {
	return s.Items
}

func (s *TransferLedger) Get(id string) (model.TransferRecord, bool) {
	item, ok := s.Items[id]
	return item, ok
}

func (s *TransferLedger) Put(item model.TransferRecord) {
	s.Items[item.ID] = item
}

func (s *TransferLedger) Count() int {
	return len(s.Items)
}
