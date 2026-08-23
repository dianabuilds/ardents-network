package authority

import (
	"encoding/json"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
)

// Prepare derives one signed existing-Name Submission from an unsigned Intent.
// It does not mutate the durable control chain: the caller must still obtain
// admission for Intent.Digest and submit the returned value through Submit.
func (control *control) Prepare(intent Intent, signer ControlSigner) (Submission, error) {
	control.mu.Lock()
	defer control.mu.Unlock()
	if control.store == nil || signer == nil {
		return Submission{}, errors.New("name Authority control preparation is unavailable")
	}
	operation, err := intent.operation()
	if err != nil {
		return Submission{}, err
	}
	current, exists := control.records[operation.Name]
	if !exists {
		return Submission{}, errors.New("name Authority predecessor is unavailable")
	}
	now := control.clock()
	op, threshold, err := operation.lifecycle(control.network, current, now)
	if err != nil || threshold {
		return Submission{}, errors.New("name Authority control intent is unavailable")
	}
	op.Parents, err = control.lineage(current, now)
	if err != nil {
		return Submission{}, err
	}
	proof, err := SignTransitionWith(control.network, current, op, controlTransitionSigner{signer: signer})
	if err != nil {
		return Submission{}, err
	}
	updated, err := applyTransition(control.network, current, op, proof, now.Unix(), control.policy)
	if err != nil {
		return Submission{}, err
	}
	successor, err := record.SignWith(control.network, updated, controlRecordSigner{signer: signer})
	if err != nil {
		return Submission{}, err
	}
	operation.AuthorityProof, operation.SuccessorRecord = proof, successor
	raw, err := json.Marshal(operation)
	if err != nil {
		return Submission{}, err
	}
	submission, err := OpenSubmission(raw)
	if err != nil || submission.digest != intent.digest {
		return Submission{}, errors.New("name Authority prepared submission is invalid")
	}
	return submission, nil
}

type controlTransitionSigner struct{ signer ControlSigner }

func (adapter controlTransitionSigner) Sign(request TransitionSigningRequest) ([]byte, error) {
	return adapter.signer.SignTransition(request)
}

type controlRecordSigner struct{ signer ControlSigner }

func (adapter controlRecordSigner) Sign(request record.RecordSigningRequest) ([]byte, error) {
	return adapter.signer.SignRecord(request)
}
