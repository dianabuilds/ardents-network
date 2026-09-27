package state

import (
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/network/state/durable"
)

// RecoveryRequiredError reports durable State that cannot safely become active
// without an explicit target-owned repair or replacement operation.
type RecoveryRequiredError struct {
	Reason string
}

func (err *RecoveryRequiredError) Error() string {
	return "network state recovery required: " + err.Reason
}

func loadGenerationChain(config config, generations map[string]durable.Generation, name string, seen map[string]bool) (verifiedEpochDecision, map[string]bool, error) {
	value, exists := generations[name]
	if !exists || seen[name] || len(seen) >= maximumEpochChain {
		return verifiedEpochDecision{}, seen, errors.New("generation chain identity, cycle, or length is invalid")
	}
	seen[name] = true
	parsed, err := parseEpoch(value.Epoch)
	if err != nil {
		return verifiedEpochDecision{}, seen, fmt.Errorf("parse generation chain Epoch: %w", err)
	}
	var previous *epochVerificationSnapshot
	if parsed.number > 1 {
		previousName := fmt.Sprintf("%x", parsed.previous)
		prior, updated, priorErr := loadGenerationChain(config, generations, previousName, seen)
		seen = updated
		if priorErr != nil {
			return verifiedEpochDecision{}, seen, priorErr
		}
		previous = &prior.Snapshot
	}
	decision, err := loadGeneration(config, value, previous)
	if err != nil {
		return verifiedEpochDecision{}, seen, err
	}
	if decision.Snapshot.Generation != name {
		return verifiedEpochDecision{}, seen, errors.New("generation identity does not match its verified digest")
	}
	return decision, seen, nil
}

func missingCurrentRecovery(generations map[string]durable.Generation) error {
	if len(generations) == 0 {
		return nil
	}
	return &RecoveryRequiredError{Reason: "current pointer is missing from a non-empty root"}
}
