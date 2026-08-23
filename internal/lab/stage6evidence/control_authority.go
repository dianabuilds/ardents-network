package stage6evidence

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

type evidenceControlAuthority struct {
	mu      sync.Mutex
	control interface {
		Apply([]byte, namespace.Proof) (string, uint64, uint64, []byte)
	}
	observed  []controlOperation
	admission []namespace.Proof
	results   []controlExecutionResult
	err       error
}

// Submit is the historical C4 bridge into the current Gateway boundary. Only
// the Stage 6 evidence package retains the old detailed Apply result for its
// archived observations; the runtime Gateway cannot observe that result.
func (authority *evidenceControlAuthority) Submit(submission namespace.Submission,
	admission namespace.Proof,
) string {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	var operation controlOperation
	raw := submission.Canonical()
	if err := json.Unmarshal(raw, &operation); err != nil {
		authority.err = err
		return "denied"
	}
	authority.observed = append(authority.observed, operation)
	authority.admission = append(authority.admission, admission)
	class, generation, revision, state := authority.control.Apply(raw, admission)
	authority.results = append(authority.results, controlExecutionResult{Class: class,
		Generation: generation, Revision: revision, State: append([]byte(nil), state...)})
	if class != "accepted" {
		authority.err = errors.New("real Name Authority control transition was denied")
	}
	if class == "accepted" {
		return "submitted"
	}
	return "denied"
}

func (authority *evidenceControlAuthority) observation() ([]controlOperation,
	[]namespace.Proof, []controlExecutionResult, error,
) {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	return append([]controlOperation(nil), authority.observed...),
		append([]namespace.Proof(nil), authority.admission...),
		append([]controlExecutionResult(nil), authority.results...), authority.err
}
