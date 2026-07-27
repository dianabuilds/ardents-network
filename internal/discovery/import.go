package discovery

import (
	"errors"
	"time"

	discoveryintake "ardents/internal/discovery/records"
)

const maxRetainedRecords = 64

func (s *Service) Import(record Record, source string) (ImportResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	trustResult, evidence := s.trust.EvaluateAtWithEvidence(record, now)
	if !trustResult.Valid {
		return ImportResult{}, errors.New(trustResult.Reason)
	}
	if source == discoveryintake.Bootstrap && !trustResult.Trusted {
		return ImportResult{
			Outcome: "rejected_untrusted",
			Reason:  "bootstrap publisher is not trusted",
		}, nil
	}
	updatedRecords, result, err := discoveryintake.ImportVerified(s.records, record, source, now, evidence)
	if err != nil {
		return ImportResult{}, err
	}
	if len(updatedRecords) > maxRetainedRecords {
		return ImportResult{
			Outcome: "rejected_capacity",
			Reason:  "retained discovery truth reached its record limit",
		}, nil
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
