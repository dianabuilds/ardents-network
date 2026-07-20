package discovery

import discoveryintake "ardents/internal/discovery/intake"

func (s *Service) upsertLocked(entry Entry) (ImportResult, error) {
	updatedRecords, result, err := discoveryintake.Upsert(s.records, entry)
	if err != nil {
		return ImportResult{}, err
	}
	s.records = updatedRecords
	return result, nil
}
