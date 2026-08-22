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
	Network            [32]byte `json:"network"`
	Nonce              [32]byte `json:"nonce"`
	Deadline           int64    `json:"deadline"`
	Name               string   `json:"name"`
	ParentName         string   `json:"parent_name"`
	Generation         uint64   `json:"generation"`
	ExpectedRevision   uint64   `json:"expected_revision"`
	ParentGeneration   uint64   `json:"parent_generation"`
	ParentRevision     uint64   `json:"parent_revision"`
	ChildGeneration    uint64   `json:"child_generation"`
	Authority          [32]byte `json:"authority"`
	SuccessorAuthority [32]byte `json:"successor_authority"`
	Target             [32]byte `json:"target"`
	LeaseNotAfter      int64    `json:"lease_not_after"`
	RecordNotAfter     int64    `json:"record_not_after"`
	PolicyNotBefore    int64    `json:"policy_not_before"`
	RecoveryNotBefore  int64    `json:"recovery_not_before"`
	PolicyID           [32]byte `json:"policy_id"`
	RecoveryStep       string   `json:"recovery_step"`
	OrderingProof      []byte   `json:"ordering_proof"`
	AuthorityProof     []byte   `json:"authority_proof"`
	RecoveryPolicy     []byte   `json:"recovery_policy"`
	RecoveryProof      []byte   `json:"recovery_proof"`
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
		if operation.RecordNotAfter <= now.UnixMilli() || operation.RecordNotAfter > current.LeaseExpiresAt*1_000 {
			return Op{}, false, errors.New("name Record expiry is invalid")
		}
		op.Kind, op.Target = "publish", operation.Target
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
			if operation.RecoveryNotBefore > now.UnixMilli() {
				return Op{}, false, errors.New("recovery resume is premature")
			}
			op.Kind, op.Target = "resume-recovery", operation.Target
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
