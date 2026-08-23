package namespace

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"time"
)

type controlOperation struct {
	Kind               string   `json:"kind"`
	OperationDigest    [32]byte `json:"operation_digest"`
	Network            [32]byte `json:"network,omitempty"`
	Nonce              [32]byte `json:"nonce,omitempty"`
	Deadline           int64    `json:"deadline,omitempty"`
	Name               string   `json:"name"`
	ParentName         string   `json:"parent_name,omitempty"`
	Generation         uint64   `json:"generation,omitempty"`
	ExpectedRevision   uint64   `json:"expected_revision,omitempty"`
	ParentGeneration   uint64   `json:"parent_generation,omitempty"`
	ParentRevision     uint64   `json:"parent_revision,omitempty"`
	ChildGeneration    uint64   `json:"child_generation,omitempty"`
	Authority          [32]byte `json:"authority,omitempty"`
	SuccessorAuthority [32]byte `json:"successor_authority,omitempty"`
	Target             [32]byte `json:"target,omitempty"`
	LeaseNotAfter      int64    `json:"lease_not_after,omitempty"`
	RecordNotAfter     int64    `json:"record_not_after,omitempty"`
	PolicyNotBefore    int64    `json:"policy_not_before,omitempty"`
	RecoveryNotBefore  int64    `json:"recovery_not_before,omitempty"`
	PolicyID           [32]byte `json:"policy_id,omitempty"`
	RecoveryStep       string   `json:"recovery_step,omitempty"`
	OrderingProof      []byte   `json:"ordering_proof,omitempty"`
	AuthorityProof     []byte   `json:"authority_proof,omitempty"`
	RecoveryPolicy     []byte   `json:"recovery_policy,omitempty"`
	RecoveryProof      []byte   `json:"recovery_proof,omitempty"`
	SuccessorRecord    []byte   `json:"successor_record,omitempty"`
}

// Submission is one validated, canonical Namespace control input. Its
// lifecycle fields remain private to Namespace; transport may bind and carry
// only this opaque value and its authenticated digest.
type Submission struct {
	raw    []byte
	digest [32]byte
}

type recoveryEnvelope struct {
	Policy RecoveryPolicy `json:"policy"`
	Proof  RecoveryProof  `json:"proof"`
}

func decodeControlOperation(raw []byte) (controlOperation, error) {
	var value controlOperation
	if len(raw) == 0 || len(raw) > 16<<10 || decodeCanonical(raw, &value) != nil ||
		value.Network != [32]byte{} || value.Nonce != [32]byte{} || value.Deadline != 0 ||
		value.OperationDigest == [32]byte{} {
		return controlOperation{}, errors.New("name Authority operation is invalid")
	}
	declared := value.OperationDigest
	value.OperationDigest = [32]byte{}
	canonical, _ := json.Marshal(value)
	if declared != controlOperationDigest(canonical) {
		return controlOperation{}, errors.New("name Authority operation digest is invalid")
	}
	value.OperationDigest = declared
	if !validControlOperation(value) {
		return controlOperation{}, errors.New("name Authority operation shape is invalid")
	}
	return value, nil
}

// OpenSubmission validates one canonical static control input for private
// transport. Dynamic transport binding is intentionally not part of the
// Namespace control representation.
func OpenSubmission(raw []byte) (Submission, error) {
	operation, err := decodeControlOperation(raw)
	if err != nil {
		return Submission{}, err
	}
	return Submission{raw: append([]byte(nil), raw...), digest: operation.OperationDigest}, nil
}

// Digest returns the operation digest that admission and the response bind.
func (submission Submission) Digest() [32]byte { return submission.digest }

// Canonical returns a copy of the sole canonical control representation for
// opaque private transport. It never exposes lifecycle fields individually.
func (submission Submission) Canonical() []byte { return append([]byte(nil), submission.raw...) }

func decodeCanonical(raw []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("name Authority operation has trailing input")
	}
	canonical, err := json.Marshal(value)
	if err != nil || !bytes.Equal(canonical, raw) {
		return errors.New("name Authority operation is non-canonical")
	}
	return nil
}

func (operation controlOperation) lifecycle(network [32]byte, current Record,
	now time.Time,
) (Op, bool, error) {
	op := Op{Name: operation.Name, ExpectedGeneration: operation.Generation,
		ExpectedRevision: operation.ExpectedRevision, Authority: current.Authority}
	switch operation.Kind {
	case "renew":
		op.Kind, op.LeaseDuration = "renew", durationUntil(now, operation.LeaseNotAfter)
	case "record":
		if operation.RecordNotAfter <= now.UnixMilli() {
			return Op{}, false, errors.New("name Record expiry is invalid")
		}
		op.Kind, op.Target, op.RecordNotAfter = "publish", operation.Target, operation.RecordNotAfter
	case "release":
		op.Kind = "release"
	case "transfer":
		op.Kind, op.SuccessorAuthority = "transfer", hex.EncodeToString(operation.SuccessorAuthority[:])
	case "policy":
		op.Kind, op.PolicyRevision = "schedule-recovery-policy", current.RecoveryPolicyRev+1
		if len(operation.RecoveryPolicy) == 0 {
			if current.RecoveryPolicy == [32]byte{} || current.RecoveryPolicyDelay <= 0 {
				return Op{}, false, errors.New("recovery Policy disable is invalid")
			}
			op.PolicyDelay = time.Duration(current.RecoveryPolicyDelay) * time.Millisecond
		} else {
			var policy RecoveryPolicy
			if decodeCanonical(operation.RecoveryPolicy, &policy) != nil || policy.Network != network ||
				policy.Name != current.Name || policy.Generation != current.Generation ||
				policy.Revision != op.PolicyRevision || policy.CurrentAuthority != authorityBytes(current.Authority) {
				return Op{}, false, errors.New("recovery Policy is invalid")
			}
			op.PolicyDigest, op.PolicyDelay = policy.Digest(), policy.Delay
			if op.PolicyDigest == [32]byte{} {
				return Op{}, false, errors.New("recovery policy participants are invalid")
			}
		}
		op.PolicyActivatesAt = operation.PolicyNotBefore
		if operation.PolicyNotBefore != now.Add(op.PolicyDelay).UnixMilli() {
			return Op{}, false, errors.New("recovery Policy activation is invalid")
		}
	case "recovery":
		if operation.PolicyID != current.RecoveryPolicy {
			return Op{}, false, errors.New("recovery Policy identifier is stale")
		}
		if operation.RecoveryStep == "resume" {
			if operation.RecoveryNotBefore > now.UnixMilli() || operation.RecordNotAfter <= now.UnixMilli() {
				return Op{}, false, errors.New("recovery resume is premature")
			}
			op.Kind, op.Target, op.RecordNotAfter = "resume-recovery", operation.Target, operation.RecordNotAfter
			return op, false, nil
		}
		var envelope recoveryEnvelope
		if decodeCanonical(operation.RecoveryProof, &envelope) != nil || envelope.Policy.Network != network ||
			envelope.Policy.Name != current.Name || envelope.Policy.Digest() != operation.PolicyID {
			return Op{}, false, errors.New("recovery proof is invalid")
		}
		authorization, err := envelope.Policy.Authorize(envelope.Proof)
		if err != nil {
			return Op{}, false, err
		}
		switch operation.RecoveryStep {
		case "initiate":
			op.Kind = "start-recovery"
			if authorization.Operation != "initiate" || operation.RecoveryNotBefore != authorization.StartedAt {
				return Op{}, false, errors.New("recovery initiation boundary is invalid")
			}
		case "cancel":
			op.Kind = "cancel-recovery"
			if authorization.Operation != "cancel" || operation.RecoveryNotBefore != now.UnixMilli() {
				return Op{}, false, errors.New("recovery cancellation boundary is invalid")
			}
		case "complete":
			op.Kind = "complete-recovery"
			if authorization.Operation != "initiate" || operation.RecoveryNotBefore != authorization.CompletesAt ||
				now.UnixMilli() < authorization.CompletesAt {
				return Op{}, false, errors.New("recovery completion boundary is invalid")
			}
		default:
			return Op{}, false, errors.New("recovery step is invalid")
		}
		op.RecoveryAuthorization = authorization
		return op, true, nil
	default:
		return Op{}, false, errors.New("name Authority operation is unavailable")
	}
	if op.Kind == "" || (op.Kind == "renew" && op.LeaseDuration <= 0) {
		return Op{}, false, errors.New("name Authority operation timing is invalid")
	}
	return op, false, nil
}

func authorityBytes(encoded string) [32]byte {
	var result [32]byte
	raw, _ := hex.DecodeString(encoded)
	copy(result[:], raw)
	return result
}

func (operation controlOperation) surface() string {
	if operation.Kind == "claim" {
		return "root-claim"
	}
	if operation.Kind == "policy" || operation.Kind == "recovery" {
		return "policy-recovery"
	}
	return "renewal-update"
}
