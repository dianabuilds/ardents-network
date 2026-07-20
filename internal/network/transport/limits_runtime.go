package transport

import (
	"fmt"
	"strings"
	"time"
)

func (s *Service) acquireNetworkOperation(payloadBytes int, provider string) (func(error), error) {
	now := timeNowUTC()
	provider = strings.TrimSpace(provider)
	s.mu.Lock()
	if payloadBytes > s.cfg.Limits.MaxMessageBytes {
		s.abuse.OversizedMessages++
		s.mu.Unlock()
		return nil, fmt.Errorf("network message exceeds %d byte limit", s.cfg.Limits.MaxMessageBytes)
	}
	if penalty := s.providerPenalties[provider]; provider != "" && penalty.bannedUntil.After(now) {
		s.mu.Unlock()
		return nil, fmt.Errorf("network provider is temporarily banned until %s", penalty.bannedUntil.UTC().Format(time.RFC3339))
	}
	if s.operationRate != nil && !s.operationRate.Allow() {
		s.abuse.RateLimitedOperations++
		s.mu.Unlock()
		return nil, fmt.Errorf("network operation rate limit exceeded")
	}
	select {
	case s.operationSlots <- struct{}{}:
		s.mu.Unlock()
		return func(err error) { s.finishNetworkOperation(provider, err) }, nil
	default:
		s.abuse.BackpressuredOperations++
		s.mu.Unlock()
		return nil, fmt.Errorf("network operation concurrency limit exceeded")
	}
}

func (s *Service) finishNetworkOperation(provider string, operationErr error) {
	<-s.operationSlots
	if provider == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if operationErr == nil {
		delete(s.providerPenalties, provider)
		return
	}
	penalty := s.providerPenalties[provider]
	penalty.failures++
	if penalty.failures >= providerFailureThreshold {
		penalty.failures = 0
		penalty.bannedUntil = timeNowUTC().Add(providerBanDuration)
	}
	s.providerPenalties[provider] = penalty
}

func (s *Service) AbuseSnapshot() AbuseSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := timeNowUTC()
	snapshot := s.abuse
	snapshot.State = "ready"
	snapshot.Limits = s.cfg.Limits
	for provider, penalty := range s.providerPenalties {
		if !penalty.bannedUntil.After(now) {
			if penalty.failures == 0 {
				delete(s.providerPenalties, provider)
			}
			continue
		}
		snapshot.BannedProviders++
	}
	if snapshot.BannedProviders > 0 {
		snapshot.State = "degraded"
		snapshot.Reason = "one or more network providers are temporarily banned after repeated failures"
	}
	return snapshot
}
