package state

import (
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/network/state/durable"
)

func loadCurrent(config config, storage *durable.Root) (*Snapshot, *candidateDecision, error) {
	current, values, err := storage.LoadState()
	if err != nil {
		return nil, nil, err
	}
	generations := make(map[string]durable.Generation, len(values))
	for _, value := range values {
		generations[value.Name] = value
	}
	if current == "" {
		if err := missingCurrentRecovery(generations); err != nil {
			return nil, nil, err
		}
		return nil, nil, nil
	}
	decision, _, err := loadGenerationChain(config, generations, current, make(map[string]bool))
	if err != nil {
		return nil, nil, err
	}
	if decision.snapshot.Generation != current {
		return nil, nil, errors.New("current pointer does not match the verified generation")
	}
	// A retained old-schema current root refuses with the typed recovery
	// outcome before it can be exposed or serve as a wave base (F-50).
	if err := classifyRetainedClosedSchema(config, decision.epoch, "current"); err != nil {
		return nil, nil, err
	}
	snapshot := decision.snapshot
	return &snapshot, &decision, nil
}

func loadGeneration(config config, generation durable.Generation, previous *Snapshot) (candidateDecision, error) {
	parsed, err := parseEpoch(generation.Epoch)
	if err != nil {
		return candidateDecision{}, fmt.Errorf("parse persisted Epoch: %w", err)
	}
	if int(parsed.cutoff) != len(generation.Inputs) {
		return candidateDecision{}, errors.New("persisted input count does not match its Epoch")
	}
	verification := config
	verification.now = parsed.validFrom
	return verifyDecision(verification, previous, generation.Epoch, generation.Inputs, nil, false)
}

func loadNamedGeneration(config config, storage *durable.Root, name string, previous *Snapshot) (candidateDecision, error) {
	_, values, err := storage.LoadState()
	if err != nil {
		return candidateDecision{}, err
	}
	for _, value := range values {
		if value.Name == name {
			return loadGeneration(config, value, previous)
		}
	}
	return candidateDecision{}, errors.New("persisted generation is missing")
}

func loadStoredChain(config config, storage *durable.Root, name string) (candidateDecision, error) {
	_, values, err := storage.LoadState()
	if err != nil {
		return candidateDecision{}, err
	}
	generations := make(map[string]durable.Generation, len(values))
	for _, value := range values {
		generations[value.Name] = value
	}
	decision, _, err := loadGenerationChain(config, generations, name, make(map[string]bool))
	return decision, err
}

func persistDecision(storage *durable.Root, decision candidateDecision, activate bool) error {
	return storage.CommitState(durable.Generation{
		Name: decision.snapshot.Generation, Epoch: decision.epochBytes,
		Inputs: decision.inputs, Activate: activate,
	})
}

func stageGeneration(storage *durable.Root, decision candidateDecision) error {
	return persistDecision(storage, decision, false)
}

// storageLimits keeps physical framing aligned with authenticated intake.
func storageLimits() durable.Limits {
	return durable.Limits{EpochBytes: maximumEpochBytes, RecordBytes: maximumRecordBytes, ClosedProfileBytes: maximumClosedProfileSize}
}
