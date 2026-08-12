package state

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/network/source"
)

func (s *networkState) serveSource(ctx context.Context, ready chan<- error) error {
	return s.config.source.Serve(ctx, ready,
		func() bool {
			s.mu.RLock()
			defer s.mu.RUnlock()
			return s.resourceProtect
		},
		func(delta int) {
			s.mu.Lock()
			if delta > 0 {
				s.activeSource++
			} else {
				s.activeSource--
			}
			s.mu.Unlock()
		},
		s.resolveDistributionRequest)
}

func (s *networkState) resolveDistributionRequest(_ context.Context, request source.Message) source.Message {
	if request.NetworkDigest != networkIdentityDigest(s.config.networkID) {
		return source.Message{Status: "bad-request"}
	}
	s.mu.RLock()
	if s.closed || s.currentDecision == nil {
		s.mu.RUnlock()
		return source.Message{Status: "busy"}
	}
	decision := *s.currentDecision
	digest := decision.epoch.digest
	s.mu.RUnlock()
	if request.Operation == "by-digest" && request.ObjectDigest != digest {
		return source.Message{Status: "not-found"}
	}
	payload, err := encodeSourceBundle(decision, request.MaterialIndex)
	if err != nil {
		return source.Message{Status: "internal"}
	}
	return source.Message{Status: "ok", ObjectDigest: digest, Payload: payload}
}
