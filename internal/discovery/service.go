package discovery

import statepkg "ardents/internal/discovery/state"

func (s *Service) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.path == "" {
		return nil
	}
	var persisted statepkg.Snapshot
	found, err := statepkg.LoadSnapshot(s.path, &persisted)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	s.records = statepkg.CloneEntries(persisted.Records)
	if persisted.State != "" {
		s.state = persisted.State
		s.reason = persisted.Reason
		return nil
	}
	if len(persisted.Records) > 0 {
		s.state = "ready"
		s.reason = ""
	}
	return nil
}

func (s *Service) Degrade(reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = "degraded"
	s.reason = reason
	return s.saveLocked()
}

func (s *Service) Ready() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = "ready"
	s.reason = ""
	return s.saveLocked()
}

func (s *Service) State() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Service) Reason() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reason
}

func (s *Service) Records() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Record, 0, len(s.records))
	for _, item := range s.records {
		out = append(out, item.Record)
	}
	return out
}

func (s *Service) Entries() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Entry, len(s.records))
	copy(out, s.records)
	return out
}

func (s *Service) markReadyLocked() {
	if s.state == "degraded" && s.reason != "" {
		return
	}
	s.state = "ready"
	s.reason = ""
}
