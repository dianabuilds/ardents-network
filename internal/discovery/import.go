package discovery

import (
	"errors"
	"time"

	discoveryintake "ardents/internal/discovery/records"
)

func (s *Service) Import(record Record, source string) (ImportResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	trustResult, evidence := s.trust.EvaluateAtWithEvidence(record, now)
	if !trustResult.Valid {
		return ImportResult{}, errors.New(trustResult.Reason)
	}
	updatedRecords, result, err := discoveryintake.ImportVerified(s.records, record, source, now, evidence)
	if err != nil {
		return ImportResult{}, err
	}
	previousRecords, previousState, previousReason := s.records, s.state, s.reason
	s.records = updatedRecords
	if !result.Applied {
		return result, nil
	}
	s.markReadyLocked()
	if err := s.saveLocked(); err != nil {
		s.records, s.state, s.reason = previousRecords, previousState, previousReason
		return ImportResult{}, err
	}
	return result, nil
}

func (s *Service) upsertLocked(entry Entry) (ImportResult, error) {
	updatedRecords, result, err := discoveryintake.Upsert(s.records, entry)
	if err != nil {
		return ImportResult{}, err
	}
	s.records = updatedRecords
	return result, nil
}
