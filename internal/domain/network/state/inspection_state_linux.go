package state

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/durable"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/internal/distribution"
)

func readRetainedInspection(cfg config, storage *durable.Inspection) (*epoch.Decision, distribution.Record, error) {
	return readRetainedHistory(cfg, storage)
}

// inspectionLocalClosure reports whether every leaf of err is a local
// close/cancellation cause of the retained inspection that owns closedSentinel.
// Each role keeps its own sentinel: a foreign role's closure or any operational
// failure is never a local cause.
func inspectionLocalClosure(err error, closedSentinel error) bool {
	if err == nil {
		return true
	}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		for _, leaf := range many.Unwrap() {
			if !inspectionLocalClosure(leaf, closedSentinel) {
				return false
			}
		}
		return true
	}
	if one, ok := err.(interface{ Unwrap() error }); ok {
		return inspectionLocalClosure(one.Unwrap(), closedSentinel)
	}
	return err == context.Canceled || err == closedSentinel
}
