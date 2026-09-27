package node

import (
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestCurrentFactsRejectsExcessCandidates(t *testing.T) {
	duty := state.NodeDuty{CandidateCount: uint8(len(state.NodeDuty{}.Candidates) + 1)}
	config := runtimeConfig{Config: Config{Current: func() (state.NodeDuty, error) { return duty, nil }}}
	if _, err := currentFacts(config); err == nil || !strings.Contains(err.Error(), "candidate count") {
		t.Fatalf("out-of-bound State duty candidate count = %v", err)
	}
	duty.CandidateCount = uint8(len(duty.Candidates))
	if _, err := currentFacts(config); err != nil {
		t.Fatalf("bounded State duty candidate count refused: %v", err)
	}
}
