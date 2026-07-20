package discovery

import (
	"time"

	discoveryresolution "ardents/internal/discovery/resolution"
)

func (s *Service) Resolve(subject, kind string) (Entry, string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return discoveryresolution.Resolve(s.records, subject, kind, time.Now().UTC())
}

func (s *Service) FindService(serviceType string) []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return discoveryresolution.FindService(s.records, serviceType, time.Now().UTC())
}

func (s *Service) Count(kind string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return discoveryresolution.Count(s.records, kind, time.Now().UTC())
}
