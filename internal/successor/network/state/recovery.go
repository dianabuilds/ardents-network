package state

import (
	"errors"
	"fmt"

	epoch2 "github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state/durable"
)

// RecoveryRequiredError reports durable State that cannot safely become active
// without an explicit target-owned repair or replacement operation.
type RecoveryRequiredError struct {
	Reason string
}

func (err *RecoveryRequiredError) Error() string {
	return "network state recovery required: " + err.Reason
}

func loadGenerationChain(config config, generations map[string]durable.Generation, name string, seen map[string]bool) (epoch2.Decision, error) {
	value, exists := generations[name]
	if !exists || seen[name] || len(seen) >= epoch2.MaxEpochChain {
		return epoch2.Decision{}, errors.New("generation chain identity, cycle, or length is invalid")
	}
	seen[name] = true
	parsed, err := epoch2.Inspect(value.Epoch)
	if err != nil {
		return epoch2.Decision{}, fmt.Errorf("parse generation chain Epoch: %w", err)
	}
	var previous *epoch2.Snapshot
	if parsed.Number > 1 {
		previousName := fmt.Sprintf("%x", parsed.Previous)
		prior, priorErr := loadGenerationChain(config, generations, previousName, seen)
		if priorErr != nil {
			return epoch2.Decision{}, priorErr
		}
		previous = &prior.Snapshot
	}
	return loadGeneration(config, value, previous)
}

func missingCurrentRecovery(generations map[string]durable.Generation) error {
	if len(generations) == 0 {
		return nil
	}
	return &RecoveryRequiredError{Reason: "current pointer is missing from a non-empty root"}
}
