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

func (authority *evidenceControlAuthority) Apply(raw []byte,
	admission namespace.Proof,
) (string, uint64, uint64, []byte) {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	var operation controlOperation
	if err := json.Unmarshal(raw, &operation); err != nil {
		authority.err = err
		return "denied", 0, 0, nil
	}
	authority.observed = append(authority.observed, operation)
	authority.admission = append(authority.admission, admission)
	class, generation, revision, state := authority.control.Apply(raw, admission)
	authority.results = append(authority.results, controlExecutionResult{Class: class,
		Generation: generation, Revision: revision, State: append([]byte(nil), state...)})
	if class != "accepted" {
		authority.err = errors.New("real Name Authority control transition was denied")
	}
	return class, generation, revision, state
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
