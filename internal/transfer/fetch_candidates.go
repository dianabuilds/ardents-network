package transfer

import (
	"ardents/internal/discovery"
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

const defaultFetchTimeout = 15 * time.Second

type fetchCandidate struct {
	record discovery.Record
	trust  discovery.TrustResult
}

type fetchCandidateSet map[string]fetchCandidate

type fetchCandidateTracker struct {
	candidates fetchCandidateSet
	failures   map[string]error
}

func trustedFetchCandidates(cfg ExchangeConfig, requester string) fetchCandidateSet {
	out := fetchCandidateSet{}
	if cfg.Discovery == nil || cfg.Trust == nil {
		return out
	}
	for _, entry := range cfg.Discovery.Entries() {
		if entry.Record.Kind() != "node" {
			continue
		}
		nodeID := entry.Record.NodeID()
		trust := cfg.Trust.Evaluate(entry.Record)
		if nodeID == "" || nodeID == requester || !trust.Usable {
			continue
		}
		out[nodeID] = fetchCandidate{record: entry.Record.Clone(), trust: trust}
	}
	return out
}

func newFetchCandidateTracker(candidates fetchCandidateSet) *fetchCandidateTracker {
	return &fetchCandidateTracker{candidates: candidates, failures: make(map[string]error)}
}

func (t *fetchCandidateTracker) contains(nodeID string) bool {
	if t == nil {
		return true
	}
	_, ok := t.candidates[nodeID]
	return ok
}

func (t *fetchCandidateTracker) canSucceed(nodeID string) bool {
	if !t.contains(nodeID) {
		return false
	}
	if t == nil {
		return true
	}
	_, failed := t.failures[nodeID]
	return !failed
}

func (t *fetchCandidateTracker) candidate(nodeID string) (fetchCandidate, bool) {
	if !t.canSucceed(nodeID) {
		return fetchCandidate{}, false
	}
	candidate, ok := t.candidates[nodeID]
	return candidate, ok
}

func (t *fetchCandidateTracker) fail(nodeID string, cause error) error {
	if t == nil || !t.contains(nodeID) {
		return nil
	}
	if _, exists := t.failures[nodeID]; !exists {
		t.failures[nodeID] = cause
	}
	if len(t.failures) != len(t.candidates) {
		return nil
	}
	nodeIDs := make([]string, 0, len(t.failures))
	for candidate := range t.failures {
		nodeIDs = append(nodeIDs, candidate)
	}
	sort.Strings(nodeIDs)
	details := make([]error, 0, len(nodeIDs))
	for _, candidate := range nodeIDs {
		details = append(details, fmt.Errorf("%s: %w", candidate, t.failures[candidate]))
	}
	return fmt.Errorf("all %d fetch candidates failed: %w", len(nodeIDs), errors.Join(details...))
}

func (t *fetchCandidateTracker) incomplete(cause error) error {
	if t == nil {
		return cause
	}
	label := "fetch interrupted"
	if errors.Is(cause, context.DeadlineExceeded) {
		label = "fetch deadline exceeded"
	}
	base := fmt.Errorf("%s after %d of %d candidates failed: %w",
		label, len(t.failures), len(t.candidates), cause)
	if len(t.failures) == 0 {
		return base
	}
	nodeIDs := make([]string, 0, len(t.failures))
	for candidate := range t.failures {
		nodeIDs = append(nodeIDs, candidate)
	}
	sort.Strings(nodeIDs)
	details := make([]error, 0, len(nodeIDs))
	for _, candidate := range nodeIDs {
		details = append(details, fmt.Errorf("%s: %w", candidate, t.failures[candidate]))
	}
	return errors.Join(base, errors.Join(details...))
}

type fetchResponseIgnoredError struct {
	err error
}

func boundedFetchContext(parent context.Context, configured time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if configured <= 0 {
		configured = defaultFetchTimeout
	}
	return context.WithTimeout(parent, configured)
}

func (e fetchResponseIgnoredError) Error() string {
	return e.err.Error()
}
