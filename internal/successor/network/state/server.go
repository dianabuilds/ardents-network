package state

import (
	"context"
	"fmt"

	source2 "github.com/dianabuilds/ardents-network/internal/successor/network/source"
)

func (s *networkState) serveSource(ctx context.Context, ready chan<- error) error {
	return s.config.source.Serve(ctx, ready,
		func() bool {
			if s.config.permitWork != nil && s.config.permitWork() != nil {
				return true
			}
			s.mu.RLock()
			defer s.mu.RUnlock()
			return s.closed || s.distribution.conflicting || s.terminalErr != nil
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

func (s *networkState) resolveDistributionRequest(_ context.Context, request source2.Message) source2.Message {
	if s.config.permitWork != nil && s.config.permitWork() != nil {
		return source2.Message{Status: "busy"}
	}
	if request.NetworkDigest != source2.NetworkDigest(s.config.networkID) {
		return source2.Message{Status: "bad-request"}
	}
	s.mu.RLock()
	if s.closed || s.current == nil || s.distribution.conflicting || s.automaticErr != nil || s.terminalErr != nil {
		s.mu.RUnlock()
		return source2.Message{Status: "busy"}
	}
	decision := *s.current
	digest := decision.Header.Digest
	s.mu.RUnlock()
	if request.Operation == "by-digest" && request.ObjectDigest != digest {
		return source2.Message{Status: "not-found"}
	}
	material, err := decision.Materialization(request.MaterialIndex)
	if err != nil {
		return source2.Message{Status: "internal"}
	}
	payload, err := source2.EncodeBundle(source2.Bundle{Epoch: decision.EpochBytes, Inputs: decision.Inputs, Materials: [][]byte{material}})
	if err != nil {
		return source2.Message{Status: "internal"}
	}
	return source2.Message{Status: "ok", ObjectDigest: digest, Payload: payload}
}
