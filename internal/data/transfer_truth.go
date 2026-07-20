package data

import (
	dataapi "ardents/internal/data/api"
	lifecyclepkg "ardents/internal/data/transfer/lifecycle"
)

func (s *Service) StartTransfer(record TransferRecord) (TransferRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record = lifecyclepkg.Start(&s.transfers, record)
	s.state = "ready"
	return record, s.saveLocked()
}

func (s *Service) CompleteTransfer(id, peer string, totalBytes int64, reason string) (TransferRecord, error) {
	return s.finishTransfer(id, "completed", peer, totalBytes, reason)
}

func (s *Service) FailTransfer(id, peer, reason string) (TransferRecord, error) {
	return s.finishTransfer(id, "failed", peer, 0, reason)
}

func (s *Service) UpdateTransferProgress(id string, progressBytes, totalBytes int64, reason string) (TransferRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := lifecyclepkg.Update(&s.transfers, id, progressBytes, totalBytes, reason)
	if err != nil {
		return TransferRecord{}, err
	}
	return item, s.saveLocked()
}

func (s *Service) GetTransfer(id string) (TransferRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return lifecyclepkg.Get(&s.transfers, id)
}

func (s *Service) ListTransfers() []TransferRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return lifecyclepkg.List(&s.transfers)
}

func (s *Service) GetTransferSnapshot(id string) (dataapi.TransferSnapshot, bool) {
	item, ok := s.GetTransfer(id)
	if !ok {
		return dataapi.TransferSnapshot{}, false
	}
	return transferSnapshot(item), true
}

func (s *Service) ListTransferSnapshots() []dataapi.TransferSnapshot {
	items := s.ListTransfers()
	out := make([]dataapi.TransferSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, transferSnapshot(item))
	}
	return out
}

func (s *Service) finishTransfer(id, state, peer string, totalBytes int64, reason string) (TransferRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, err := lifecyclepkg.Finish(&s.transfers, id, state, peer, totalBytes, reason)
	if err != nil {
		return TransferRecord{}, err
	}
	s.state = "ready"
	return item, s.saveLocked()
}
