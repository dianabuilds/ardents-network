package state

import (
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/state/durable"
)

func loadCurrent(config config, storage *durable.Root) (*Snapshot, *epoch.Decision, error) {
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
	if decision.Snapshot.Generation != current {
		return nil, nil, errors.New("current pointer does not match the verified generation")
	}
	// A retained old-schema current root refuses with the typed recovery
	// outcome before it can be exposed or serve as a wave base (F-50).
	if err := classifyRetainedClosedSchema(config, decision.Header, "current"); err != nil {
		return nil, nil, err
	}
	snapshot := snapshotFromEpoch(decision.Snapshot)
	return &snapshot, &decision, nil
}

func loadGeneration(config config, generation durable.Generation, previous *epoch.Snapshot) (epoch.Decision, error) {
	parsed, err := epoch.Inspect(generation.Epoch)
	if err != nil {
		return epoch.Decision{}, fmt.Errorf("parse persisted Epoch: %w", err)
	}
	if int(parsed.Cutoff) != len(generation.Inputs) {
		return epoch.Decision{}, errors.New("persisted input count does not match its Epoch")
	}
	verification := config
	verification.now = parsed.ValidFrom
	return verifyDecision(verification, previous, generation.Epoch, generation.Inputs, nil, false)
}

func loadNamedGeneration(config config, storage *durable.Root, name string, previous *epoch.Snapshot) (epoch.Decision, error) {
	_, values, err := storage.LoadState()
	if err != nil {
		return epoch.Decision{}, err
	}
	for _, value := range values {
		if value.Name == name {
			return loadGeneration(config, value, previous)
		}
	}
	return epoch.Decision{}, errors.New("persisted generation is missing")
}

func loadStoredChain(config config, storage *durable.Root, name string) (epoch.Decision, error) {
	_, values, err := storage.LoadState()
	if err != nil {
		return epoch.Decision{}, err
	}
	generations := make(map[string]durable.Generation, len(values))
	for _, value := range values {
		generations[value.Name] = value
	}
	decision, _, err := loadGenerationChain(config, generations, name, make(map[string]bool))
	return decision, err
}

func persistDecision(storage *durable.Root, decision epoch.Decision, activate bool) error {
	return storage.CommitState(durable.Generation{
		Name: decision.Snapshot.Generation, Epoch: decision.EpochBytes,
		Inputs: decision.Inputs, Activate: activate,
	})
}

func stageGeneration(storage *durable.Root, decision epoch.Decision) error {
	return persistDecision(storage, decision, false)
}

// storageLimits keeps physical framing aligned with authenticated intake.
func storageLimits() durable.Limits {
	return durable.Limits{EpochBytes: maximumEpochBytes, RecordBytes: maximumRecordBytes, ClosedProfileBytes: closedprofile.MaxSize}
}
