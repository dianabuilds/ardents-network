package processfixture

import (
	"fmt"
	"sync"
)

// AdmissionPresentations records results from genuine Receiving owners. Token
// digests detect repeated presentations across duties; JOIN counts describe
// only the selected Rendezvous.
type AdmissionPresentations struct {
	mu            sync.Mutex
	accepted      int
	joinAttempts  int
	repeatedToken bool
	reusedChannel bool
	tokens        map[[32]byte]bool
	joinChannels  map[[32]byte]bool
}

// Observe records the detached facts after the original owner has decided.
func (evidence *AdmissionPresentations) Observe(selectedJoin bool, binding, token [32]byte, outcome error) {
	evidence.mu.Lock()
	defer evidence.mu.Unlock()
	if evidence.tokens == nil {
		evidence.tokens = make(map[[32]byte]bool)
		evidence.joinChannels = make(map[[32]byte]bool)
	}
	evidence.repeatedToken = evidence.repeatedToken || evidence.tokens[token]
	evidence.tokens[token] = true
	if !selectedJoin {
		return
	}
	evidence.joinAttempts++
	evidence.reusedChannel = evidence.reusedChannel || evidence.joinChannels[binding] || binding == [32]byte{}
	evidence.joinChannels[binding] = true
	if outcome == nil {
		evidence.accepted++
	}
}

// Check requires the expected number of original, successful JOINs and no
// repeated token digests or missing or reused JOIN channel identities.
func (evidence *AdmissionPresentations) Check(expected int) error {
	evidence.mu.Lock()
	defer evidence.mu.Unlock()
	if evidence.accepted != expected || evidence.joinAttempts != expected || evidence.repeatedToken || evidence.reusedChannel {
		return fmt.Errorf("actual JOIN admissions=%d presentations=%d repeated token=%t reused channel=%t, want %d fresh admissions",
			evidence.accepted, evidence.joinAttempts, evidence.repeatedToken, evidence.reusedChannel, expected)
	}
	return nil
}
