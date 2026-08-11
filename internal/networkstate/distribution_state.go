package networkstate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maximumDistributionGenerations = 4096

type distributionState struct {
	sequence            uint64
	epochFloor          uint64
	epochDigest         [32]byte
	trustedTimeFloor    int64
	conflicting         bool
	consecutiveFailures uint64
	backoffLevel        byte
	nextAutomatic       int64
	history             [][32]byte
	cycleID             uint64
	cycleActive         bool
	cyclePurpose        byte
	cycleStarted        int64
	cycleDeadline       int64
	attempts            [4]byte
	outcomes            [4]byte
	requestedDigests    [2][32]byte
	observedEpochs      [4]uint64
	observedDigests     [4][32]byte
	pendingDigest       [32]byte
	pendingValidFrom    int64
	cycleSeed           [32]byte
	sourceOrder         [2]byte
}

func (s *store) loadDistributionState() error {
	directory := filepath.Join(s.config.root, "distribution")
	generations := filepath.Join(directory, "generations")
	if err := os.MkdirAll(generations, 0o700); err != nil {
		return fmt.Errorf("create distribution state: %w", err)
	}
	if err := syncDirectory(generations); err != nil {
		return fmt.Errorf("sync distribution generations: %w", err)
	}
	if err := syncDirectory(directory); err != nil {
		return fmt.Errorf("sync distribution root: %w", err)
	}
	if err := syncDirectory(s.config.root); err != nil {
		return fmt.Errorf("sync owned state root: %w", err)
	}
	if err := cleanupDistributionStaging(directory, generations); err != nil {
		return err
	}
	pointer, err := readBoundedFile(filepath.Join(directory, "current"), 65)
	if os.IsNotExist(err) {
		entries, scanErr := readBoundedDirectory(generations, maximumDistributionGenerations+1)
		if scanErr != nil || len(entries) != 0 {
			return errors.New("distribution state lacks its current pointer")
		}
		if s.current != nil {
			s.distribution.epochFloor = s.current.Epoch
			s.distribution.epochDigest = s.current.Digest
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read distribution pointer: %w", err)
	}
	name := strings.TrimSuffix(string(pointer), "\n")
	if string(pointer) != name+"\n" || !generationName.MatchString(name) {
		return errors.New("distribution pointer is not canonical")
	}
	raw, err := readBoundedFile(filepath.Join(directory, "generations", name, "state.bin"), 4096)
	if err != nil {
		return fmt.Errorf("read distribution generation: %w", err)
	}
	state, err := decodeDistributionState(raw)
	if err != nil || distributionDigest(raw) != name {
		return errors.New("distribution generation is invalid")
	}
	if state.epochFloor != 0 && (s.current == nil || state.epochFloor > s.current.Epoch ||
		state.epochFloor == s.current.Epoch && state.epochDigest != s.current.Digest) {
		if err := s.recoverDistributionActive(state); err != nil {
			return err
		}
	}
	if s.current != nil && state.epochFloor < s.current.Epoch {
		return errors.New("distribution security state is older than the active generation")
	}
	s.distribution = state
	return s.recoverPendingState()
}

func (s *store) recoverDistributionActive(state distributionState) error {
	name := fmt.Sprintf("%x", state.epochDigest)
	decision, _, err := loadGenerationChain(s.config, name, make(map[string]bool), true)
	if err != nil {
		return fmt.Errorf("recover distribution active generation: %w", err)
	}
	if decision.epoch.number != state.epochFloor || decision.epoch.digest != state.epochDigest {
		return errors.New("distribution active identity disagrees with its generation")
	}
	if err := replaceCurrent(s.config.root, name); err != nil {
		return fmt.Errorf("repair active generation pointer: %w", err)
	}
	snapshot := decision.snapshot
	s.current, s.currentDecision = &snapshot, &decision
	return nil
}

func cleanupDistributionStaging(root, generations string) error {
	for directory, prefix := range map[string]string{root: ".current-", generations: ".stage-"} {
		entries, err := readBoundedDirectory(directory, maximumDistributionGenerations+2)
		if err != nil {
			return fmt.Errorf("scan distribution staging: %w", err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), prefix) {
				if err := os.RemoveAll(filepath.Join(directory, entry.Name())); err != nil {
					return fmt.Errorf("remove distribution staging: %w", err)
				}
			}
		}
	}
	return nil
}

func (s *store) commitDistribution(state distributionState) error {
	raw := encodeDistributionState(state)
	name := distributionDigest(raw)
	root := filepath.Join(s.config.root, "distribution")
	generations := filepath.Join(root, "generations")
	entries, err := readBoundedDirectory(generations, maximumDistributionGenerations+1)
	if err != nil || len(entries) >= maximumDistributionGenerations {
		return errors.New("distribution generation bound is exhausted")
	}
	staging, err := os.MkdirTemp(generations, ".stage-")
	if err != nil {
		return fmt.Errorf("stage distribution generation: %w", err)
	}
	defer os.RemoveAll(staging)
	if err := writeSynced(filepath.Join(staging, "state.bin"), raw); err != nil {
		return err
	}
	if err := syncDirectory(staging); err != nil {
		return err
	}
	if err := os.Rename(staging, filepath.Join(generations, name)); err != nil && !os.IsExist(err) {
		return fmt.Errorf("publish distribution generation: %w", err)
	}
	if err := syncDirectory(generations); err != nil {
		return err
	}
	if err := replaceNamedPointer(root, "current", name); err != nil {
		return err
	}
	s.distribution = state
	return nil
}

func (s *store) commitActiveDecision(decision candidateDecision, state distributionState) error {
	if err := publishGeneration(s.config.root, decision); err != nil {
		return err
	}
	state.epochFloor, state.epochDigest = decision.epoch.number, decision.epoch.digest
	if err := s.commitDistribution(state); err != nil {
		return err
	}
	snapshot := decision.snapshot
	s.current, s.currentDecision = &snapshot, &decision
	return replaceCurrent(s.config.root, decision.snapshot.Generation)
}
