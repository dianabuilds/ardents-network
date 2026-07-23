package discovery

func (s *Service) Snapshot() ([]Entry, string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return CloneEntries(s.records), s.state, s.reason
}

func (s *Service) Restore(records []Entry, state, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if state == "" {
		state = "ready"
	}
	candidate := Snapshot{SchemaVersion: 1, Records: CloneEntries(records), State: state, Reason: reason}
	if err := validateSnapshotState(candidate); err != nil {
		return err
	}
	validated, err := validateSnapshotEntries(candidate.Records)
	if err != nil {
		return err
	}
	previousRecords, previousState, previousReason := s.records, s.state, s.reason
	s.records, s.state, s.reason = validated, state, reason
	if err := s.saveLocked(); err != nil {
		s.records, s.state, s.reason = previousRecords, previousState, previousReason
		return err
	}
	return nil
}
