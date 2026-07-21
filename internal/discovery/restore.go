package discovery

func (s *Service) Snapshot() ([]Entry, string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return CloneEntries(s.records), s.state, s.reason
}

func (s *Service) Restore(records []Entry, state, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records = CloneEntries(records)
	if state == "" {
		state = "ready"
	}
	s.state = state
	s.reason = reason
	return s.saveLocked()
}
