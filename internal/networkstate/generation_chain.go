package networkstate

import (
	"errors"
	"fmt"
	"path/filepath"
)

func verifyEpochChain(current *Snapshot, epoch epochEnvelope) error {
	if epoch.number > maximumEpochChain {
		return errors.New("epoch exceeds the S1-0 retained chain bound")
	}
	var zero [32]byte
	if current == nil {
		if epoch.number != 1 || epoch.previous != zero {
			return errors.New("genesis epoch chain is invalid")
		}
		return nil
	}
	if epoch.number != current.Epoch+1 || epoch.previous != current.Digest {
		return errors.New("epoch transition does not extend current state")
	}
	return nil
}

func loadGenerationChain(config config, name string, seen map[string]bool, current bool) (candidateDecision, map[string]bool, error) {
	if !generationName.MatchString(name) || seen[name] || len(seen) >= maximumEpochChain {
		return candidateDecision{}, seen, errors.New("generation chain identity, cycle, or length is invalid")
	}
	seen[name] = true
	epochBytes, err := readBoundedFile(filepath.Join(config.root, "generations", name, "epoch.bin"), maximumEpochBytes)
	if err != nil {
		return candidateDecision{}, seen, fmt.Errorf("read generation chain epoch: %w", err)
	}
	epoch, err := parseEpoch(epochBytes)
	if err != nil {
		return candidateDecision{}, seen, fmt.Errorf("parse generation chain epoch: %w", err)
	}
	var previous *Snapshot
	if epoch.number > 1 {
		previousName := fmt.Sprintf("%x", epoch.previous)
		prior, updated, priorErr := loadGenerationChain(config, previousName, seen, false)
		seen = updated
		if priorErr != nil {
			return candidateDecision{}, seen, priorErr
		}
		previous = &prior.snapshot
	}
	decision, err := loadGeneration(config, name, previous, current)
	if err != nil {
		return candidateDecision{}, seen, err
	}
	if decision.snapshot.Generation != name {
		return candidateDecision{}, seen, errors.New("generation directory does not match its verified digest")
	}
	return decision, seen, nil
}

func recoverMissingCurrent(config config) (*Snapshot, error) {
	generations := filepath.Join(config.root, "generations")
	entries, err := readBoundedDirectory(generations, maximumEpochChain)
	if err != nil {
		return nil, fmt.Errorf("scan generations without current pointer: %w", err)
	}
	if len(entries) == 0 {
		return nil, nil
	}
	referenced := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !generationName.MatchString(entry.Name()) {
			return nil, errors.New("generation directory contains an invalid entry")
		}
		epochBytes, readErr := readBoundedFile(filepath.Join(generations, entry.Name(), "epoch.bin"), maximumEpochBytes)
		if readErr != nil {
			return nil, fmt.Errorf("read orphan generation epoch: %w", readErr)
		}
		epoch, parseErr := parseEpoch(epochBytes)
		if parseErr != nil {
			return nil, fmt.Errorf("parse orphan generation epoch: %w", parseErr)
		}
		if epoch.number > 1 {
			referenced[fmt.Sprintf("%x", epoch.previous)] = true
		}
	}
	var tip string
	for _, entry := range entries {
		if referenced[entry.Name()] {
			continue
		}
		if tip != "" {
			return nil, errors.New("missing current pointer has multiple generation tips")
		}
		tip = entry.Name()
	}
	if tip == "" {
		return nil, errors.New("missing current pointer has no acyclic generation tip")
	}
	decision, seen, err := loadGenerationChain(config, tip, make(map[string]bool), true)
	if err != nil {
		return nil, err
	}
	if len(seen) != len(entries) {
		return nil, errors.New("missing current pointer has an orphan generation branch")
	}
	if err := replaceCurrent(config.root, tip); err != nil {
		return nil, fmt.Errorf("recover current pointer: %w", err)
	}
	snapshot := decision.snapshot
	return &snapshot, nil
}
