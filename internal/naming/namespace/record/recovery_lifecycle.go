package record

import (
	"encoding/hex"
	"time"
)

const (
	minimumRecoveryDelay = 72 * time.Hour
	maximumRecoveryDelay = 30 * 24 * time.Hour
)

func applyRotate(current *Record, now int64, op Op) (Record, error) {
	if err := requireCurrent(current, op, op.Kind); err != nil {
		return Record{}, err
	}
	if op.Authority != current.Authority || op.SuccessorAuthority == "" || op.SuccessorAuthority == current.Authority {
		return Record{}, transitionError{Action: op.Kind, Reason: "current and successor authorities are invalid"}
	}
	if ok, reason := liveLease(*current, now); !ok {
		return Record{}, transitionError{Action: op.Kind, Reason: reason}
	}
	result := *current
	result.Revision++
	result.Authority = op.SuccessorAuthority
	return result, nil
}

func applyRecoveryPolicy(current *Record, seconds, milliseconds int64, op Op) (Record, error) {
	if err := requireCurrent(current, op, op.Kind); err != nil {
		return Record{}, err
	}
	result := *current
	switch op.Kind {
	case opScheduleRecoveryPolicy:
		if ok, _ := liveLease(*current, seconds); !ok {
			return Record{}, transitionError{Action: op.Kind, Reason: "policy change requires a live current Lease"}
		}
		if op.Authority != current.Authority ||
			current.PendingPolicyRev != 0 || op.PolicyRevision != current.RecoveryPolicyRev+1 ||
			op.PolicyDelay < minimumRecoveryDelay || op.PolicyDelay > maximumRecoveryDelay ||
			op.PolicyActivatesAt != milliseconds+op.PolicyDelay.Milliseconds() ||
			(op.PolicyDigest == [32]byte{} && current.RecoveryPolicy == [32]byte{}) {
			return Record{}, transitionError{Action: op.Kind, Reason: "policy change is invalid or already pending"}
		}
		result.PendingPolicy, result.PendingPolicyRev = op.PolicyDigest, op.PolicyRevision
		result.PendingPolicyDelay = op.PolicyDelay.Milliseconds()
		result.PolicyActivatesAt = op.PolicyActivatesAt
	case opActivateRecoveryPolicy:
		if current.PendingPolicyRev == 0 || milliseconds < current.PolicyActivatesAt || current.Recovery != recoveryStable {
			return Record{}, transitionError{Action: op.Kind, Reason: "policy change is not eligible"}
		}
		result.RecoveryPolicy, result.RecoveryPolicyRev = current.PendingPolicy, current.PendingPolicyRev
		result.RecoveryPolicyDelay = current.PendingPolicyDelay
		result.PendingPolicy, result.PendingPolicyRev, result.PendingPolicyDelay, result.PolicyActivatesAt =
			[32]byte{}, 0, 0, 0
	}
	result.Revision++
	return result, nil
}

func applyRecovery(current *Record, seconds, milliseconds int64, op Op) (Record, error) {
	if err := requireCurrent(current, op, op.Kind); err != nil {
		return Record{}, err
	}
	result := *current
	authorization := op.RecoveryAuthorization
	switch op.Kind {
	case opStartRecovery:
		if ok, _ := liveLease(*current, seconds); !ok {
			return Record{}, transitionError{Action: op.Kind, Reason: "recovery requires a live current Lease"}
		}
		if !authorization.Verified() || authorization.Operation != "initiate" ||
			current.RecoveryPolicy == [32]byte{} || authorization.PolicyDigest != current.RecoveryPolicy ||
			authorization.PolicyRevision != current.RecoveryPolicyRev || authorization.StartedAt > milliseconds ||
			milliseconds-authorization.StartedAt > 30*time.Second.Milliseconds() ||
			hex.EncodeToString(authorization.Successor[:]) == current.Authority {
			return Record{}, transitionError{Action: op.Kind, Reason: "recovery authorization is invalid"}
		}
		result.Recovery = recoveryPending
		result.RecoveryOperation, result.RecoverySuccessor = authorization.OperationID, authorization.Successor
		result.RecoveryStartedAt, result.RecoveryExpiresAt = authorization.StartedAt, authorization.CompletesAt
	case opCancelRecovery:
		if !matchingPendingRecovery(current, authorization, "cancel") || milliseconds >= current.RecoveryExpiresAt ||
			current.Authority == hex.EncodeToString(current.RecoverySuccessor[:]) {
			return Record{}, transitionError{Action: op.Kind, Reason: "recovery cancellation is invalid"}
		}
		clearPendingRecovery(&result)
	case opCompleteRecovery:
		if !matchingPendingRecovery(current, authorization, "initiate") || milliseconds < current.RecoveryExpiresAt ||
			current.Authority == hex.EncodeToString(current.RecoverySuccessor[:]) {
			return Record{}, transitionError{Action: op.Kind, Reason: "recovery is not eligible for completion"}
		}
		completePendingRecovery(&result)
	case opResumeRecovery:
		if current.Recovery != recoveryStable || current.Consistency != consistencyUnavailable ||
			current.Target != [32]byte{} || op.Authority != current.Authority || op.Target == [32]byte{} ||
			op.RecordNotAfter <= milliseconds || op.RecordNotAfter > leaseNotAfter(*current)*1_000 {
			return Record{}, transitionError{Action: op.Kind, Reason: "fresh successor Record is invalid"}
		}
		result.Target, result.RecordNotAfter = op.Target, op.RecordNotAfter
		result.Consistency = consistencyCurrent
	}
	result.Revision++
	return result, nil
}

func matchingPendingRecovery(current *Record, authorization Authorization, operation string) bool {
	return authorization.Verified() && authorization.Operation == operation &&
		authorization.PolicyDigest == current.RecoveryPolicy &&
		authorization.PolicyRevision == current.RecoveryPolicyRev &&
		authorization.OperationID == current.RecoveryOperation &&
		authorization.Successor == current.RecoverySuccessor &&
		authorization.StartedAt == current.RecoveryStartedAt &&
		authorization.CompletesAt == current.RecoveryExpiresAt
}

func clearPendingRecovery(record *Record) {
	record.Recovery = recoveryStable
	record.RecoveryOperation, record.RecoverySuccessor = [32]byte{}, [32]byte{}
	record.RecoveryStartedAt, record.RecoveryExpiresAt = 0, 0
}

func completePendingRecovery(record *Record) {
	record.Authority = hex.EncodeToString(record.RecoverySuccessor[:])
	record.Target, record.RecordNotAfter = [32]byte{}, 0
	record.Consistency = consistencyUnavailable
	clearPendingRecovery(record)
}
