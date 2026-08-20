package namelease

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
		return record.RecoveryExpiresAt == 0
	}
	return record.RecoveryExpiresAt > 0
}

func hasRequiredParent(record Record) bool {
	return !strings.Contains(record.Name, ".") || record.ParentName != ""
}
