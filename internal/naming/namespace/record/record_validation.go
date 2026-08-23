package record

import (
	"errors"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/naming"
)

func validOperationName(raw string) error {
	name, err := naming.Parse(raw)
	if err != nil {
		return err
	}
	if string(name) != raw {
		return errors.New("service name is not canonical")
	}
	return nil
}

func requireCurrent(current *Record, op Op, action string) error {
	if current == nil {
		return transitionError{Action: action, Reason: "name is unclaimed"}
	}
	if current.Name != op.Name {
		return transitionError{Action: action, Reason: "record name mismatch"}
	}
	if err := validateRecord(*current); err != nil {
		return transitionError{Action: action, Reason: "current record is invalid: " + err.Error()}
	}
	if op.ExpectedGeneration != current.Generation || op.ExpectedRevision != current.Revision {
		return transitionError{Action: action, Reason: "generation or revision is stale"}
	}
	return nil
}

func validStates(record Record) bool {
	lease := record.Lease == leaseActive || record.Lease == leaseGrace || record.Lease == leaseReleased
	consistency := record.Consistency == consistencyCurrent || record.Consistency == consistencyConflict ||
		record.Consistency == consistencyFork || record.Consistency == consistencyUnavailable
	recovery := record.Recovery == recoveryStable || record.Recovery == recoveryPending
	return lease && consistency && recovery
}

func validRecordLifetimes(record Record) bool {
	switch record.Lease {
	case leaseActive, leaseGrace:
		if record.LeaseExpiresAt <= 0 || record.GraceExpiresAt < record.LeaseExpiresAt {
			return false
		}
	case leaseReleased:
		if record.LeaseExpiresAt != 0 || record.GraceExpiresAt != 0 {
			return false
		}
	}
	if record.Recovery == recoveryStable {
		return record.RecoveryExpiresAt == 0 && record.RecoveryStartedAt == 0 &&
			record.RecoveryOperation == [32]byte{} && record.RecoverySuccessor == [32]byte{}
	}
	return record.RecoveryExpiresAt > record.RecoveryStartedAt && record.RecoveryStartedAt > 0 &&
		record.RecoveryOperation != [32]byte{} && record.RecoverySuccessor != [32]byte{} &&
		record.RecoveryPolicy != [32]byte{} && record.RecoveryPolicyRev > 0 &&
		record.RecoveryExpiresAt-record.RecoveryStartedAt == record.RecoveryPolicyDelay
}

func validRecoveryBindings(record Record) bool {
	if record.RecoveryPolicyRev == 0 && (record.RecoveryPolicy != [32]byte{} || record.RecoveryPolicyDelay != 0) {
		return false
	}
	if record.RecoveryPolicyRev > 0 && (record.RecoveryPolicyDelay < minimumRecoveryDelay.Milliseconds() ||
		record.RecoveryPolicyDelay > maximumRecoveryDelay.Milliseconds()) {
		return false
	}
	if record.PendingPolicyRev == 0 {
		return record.PendingPolicy == [32]byte{} && record.PendingPolicyDelay == 0 && record.PolicyActivatesAt == 0
	}
	return record.PendingPolicyRev > record.RecoveryPolicyRev &&
		record.PendingPolicyDelay >= minimumRecoveryDelay.Milliseconds() &&
		record.PendingPolicyDelay <= maximumRecoveryDelay.Milliseconds() && record.PolicyActivatesAt > 0
}

func hasRequiredParent(record Record) bool {
	return !strings.Contains(record.Name, ".") || record.ParentName != ""
}

// Validate verifies one canonical Record value.
func Validate(value Record) error { return validateRecord(value) }

// ValidateParents verifies a complete immediate-parent-to-root lineage.
func ValidateParents(child string, parents []Record, now int64) error {
	_, err := validateParents(child, parents, now)
	return err
}
