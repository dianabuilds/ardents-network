package discovery

import statepkg "ardents/internal/discovery/state"

func (s *Service) Snapshot() ([]Entry, string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return statepkg.CloneEntries(s.records), s.state, s.reason
}

func (s *Service) Restore(records []Entry, state, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records = statepkg.CloneEntries(records)
	if state == "" {
		state = "ready"
	}
	s.state = state
	s.reason = reason
	return s.saveLocked()
}
