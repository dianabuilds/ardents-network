package state

import (
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile"
	epoch2 "github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	durable2 "github.com/dianabuilds/ardents-network/internal/successor/network/state/durable"
)

func loadCurrent(config config, storage *durable2.Root) (*epoch2.Decision, error) {
	current, values, err := storage.LoadState()
	if err != nil {
		return nil, err
	}
	generations := make(map[string]durable2.Generation, len(values))
	for _, value := range values {
		generations[value.Name] = value
	}
	if current == "" {
		if err := missingCurrentRecovery(generations); err != nil {
			return nil, err
		}
		return nil, nil
	}
	decision, err := loadGenerationChain(config, generations, current, make(map[string]bool))
	if err != nil {
		return nil, err
	}
	// A retained old-schema current root refuses with the typed recovery
	// outcome before it can be exposed or serve as a wave base (F-50).
	if err := classifyRetainedClosedSchema(config, decision.Header, "current"); err != nil {
		return nil, err
	}
	return &decision, nil
}

func loadGeneration(config config, generation durable2.Generation, previous *epoch2.Snapshot) (epoch2.Decision, error) {
	parsed, err := epoch2.Inspect(generation.Epoch)
	if err != nil {
		return epoch2.Decision{}, fmt.Errorf("parse persisted Epoch: %w", err)
	}
	if int(parsed.Cutoff) != len(generation.Inputs) {
		return epoch2.Decision{}, errors.New("persisted input count does not match its Epoch")
	}
	verification := config
	verification.now = parsed.ValidFrom
	decision, err := verifyDecision(verification, previous, generation.Epoch, generation.Inputs, nil, false)
	if err != nil {
		return epoch2.Decision{}, err
	}
	if decision.Snapshot.Generation != generation.Name {
		return epoch2.Decision{}, errors.New("generation identity does not match its verified digest")
	}
	return decision, nil
}

func loadNamedGeneration(config config, storage *durable2.Root, name string, previous *epoch2.Snapshot) (epoch2.Decision, error) {
	_, values, err := storage.LoadState()
	if err != nil {
		return epoch2.Decision{}, err
	}
	for _, value := range values {
		if value.Name == name {
			return loadGeneration(config, value, previous)
		}
	}
	return epoch2.Decision{}, errors.New("persisted generation is missing")
}

func loadStoredChain(config config, storage *durable2.Root, name string) (epoch2.Decision, error) {
	_, values, err := storage.LoadState()
	if err != nil {
		return epoch2.Decision{}, err
	}
	generations := make(map[string]durable2.Generation, len(values))
	for _, value := range values {
		generations[value.Name] = value
	}
	decision, err := loadGenerationChain(config, generations, name, make(map[string]bool))
	return decision, err
}

func persistDecision(storage *durable2.Root, decision epoch2.Decision, activate bool) error {
	return storage.CommitState(durable2.Generation{
		Name: decision.Snapshot.Generation, Epoch: decision.EpochBytes,
		Inputs: decision.Inputs, Activate: activate,
	})
}

func stageGeneration(storage *durable2.Root, decision epoch2.Decision) error {
	return persistDecision(storage, decision, false)
}

// storageLimits keeps physical framing aligned with authenticated intake.
func storageLimits() durable2.Limits {
	return durable2.Limits{EpochBytes: maximumEpochBytes, RecordBytes: maximumRecordBytes, ClosedProfileBytes: closedprofile.MaxSize}
}
