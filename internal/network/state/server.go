package state

import (
	"context"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/network/source"
)

func (s *networkState) serveSource(ctx context.Context, ready chan<- error) error {
	return s.config.source.Serve(ctx, ready,
		func() bool {
			s.mu.RLock()
			defer s.mu.RUnlock()
			return s.resourceProtect
		},
		s.sourceConnectionActive,
		s.resolveDistributionRequest)
}

// A response can keep predecessor bytes after a successor is published. The
// last accepted handler releases those predecessor guards after it closes.
func (s *networkState) sourceConnectionActive(delta int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if delta > 0 {
		s.activeSource++
		return
	}
	s.activeSource--
	if s.activeSource != 0 || s.closed || len(s.servingPredecessors) == 0 {
		return
	}
	if err := s.retainSourceServer(); err != nil {
		s.terminalErr = fmt.Errorf("release joined Source predecessors: %w", err)
		s.retireStateLocked()
		return
	}
	s.servingPredecessors = nil
}

func (s *networkState) resolveDistributionRequest(_ context.Context, request source.Message) source.Message {
	if request.NetworkDigest != source.NetworkDigest(s.config.networkID) {
		return source.Message{Status: "bad-request"}
	}
	s.mu.RLock()
	if s.closed || s.current == nil || s.distribution.conflicting || s.automaticErr != nil || s.resourceErr != nil {
		s.mu.RUnlock()
		return source.Message{Status: "busy"}
	}
	decision := *s.current
	digest := decision.Header.Digest
	s.mu.RUnlock()
	if request.Operation == "by-digest" && request.ObjectDigest != digest {
		return source.Message{Status: "not-found"}
	}
	material, err := decision.Materialization(request.MaterialIndex)
	if err != nil {
		return source.Message{Status: "internal"}
	}
	payload, err := source.EncodeBundle(source.Bundle{Epoch: decision.EpochBytes, Inputs: decision.Inputs, Materials: [][]byte{material}})
	if err != nil {
		return source.Message{Status: "internal"}
	}
	return source.Message{Status: "ok", ObjectDigest: digest, Payload: payload}
}
