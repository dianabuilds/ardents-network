package discovery

import (
	"time"

	discoveryintake "ardents/internal/discovery/intake"
)

func (s *Service) Import(record Record, source string) (ImportResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedRecords, result, err := discoveryintake.Import(s.records, record, source, time.Now().UTC())
	if err != nil {
		return ImportResult{}, err
	}
	s.records = updatedRecords
	if !result.Applied {
		return result, nil
	}
	s.markReadyLocked()
	return result, s.saveLocked()
}
