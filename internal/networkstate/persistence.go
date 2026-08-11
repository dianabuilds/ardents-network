package networkstate

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var generationName = regexp.MustCompile(`^[0-9a-f]{64}$`)

func loadCurrent(config config) (*Snapshot, error) {
	pointer, err := readBoundedFile(filepath.Join(config.root, "current"), 65)
	if os.IsNotExist(err) {
		return recoverMissingCurrent(config)
	}
	if err != nil {
		return nil, fmt.Errorf("read current pointer: %w", err)
	}
	name := strings.TrimSuffix(string(pointer), "\n")
	if string(pointer) != name+"\n" || !generationName.MatchString(name) {
		return nil, errors.New("current pointer is not canonical")
	}
	decision, _, err := loadGenerationChain(config, name, make(map[string]bool), true)
	if err != nil {
		return nil, err
	}
	if decision.snapshot.Generation != name {
		return nil, errors.New("current pointer does not match the verified generation")
	}
	snapshot := decision.snapshot
	return &snapshot, nil
}

func loadGeneration(config config, name string, previous *Snapshot, current bool) (candidateDecision, error) {
	directory := filepath.Join(config.root, "generations", name)
	epochBytes, err := readBoundedFile(filepath.Join(directory, "epoch.bin"), maximumEpochBytes)
	if err != nil {
		return candidateDecision{}, fmt.Errorf("read current epoch: %w", err)
	}
	epoch, err := parseEpoch(epochBytes)
	if err != nil {
		return candidateDecision{}, fmt.Errorf("parse current epoch: %w", err)
	}
	inputs := make([][]byte, epoch.cutoff)
	for index := range inputs {
		path := filepath.Join(directory, "inputs", fmt.Sprintf("%04d.bin", index))
		inputs[index], err = readBoundedFile(path, maximumRecordBytes)
		if err != nil {
			return candidateDecision{}, fmt.Errorf("read current input %d: %w", index, err)
		}
	}
	entries, err := readBoundedDirectory(filepath.Join(directory, "inputs"), 64)
	if err != nil {
		return candidateDecision{}, fmt.Errorf("scan current inputs: %w", err)
	}
	if len(entries) != len(inputs) {
		return candidateDecision{}, errors.New("current generation has an unexpected input file")
	}
	verificationConfig := config
	if !current {
		verificationConfig.now = epoch.validFrom
	}
	return verifyDecision(verificationConfig, previous, epochBytes, inputs, nil, false)
}

func commitGeneration(root string, decision candidateDecision) error {
	generations := filepath.Join(root, "generations")
	staging, err := os.MkdirTemp(generations, ".stage-")
	if err != nil {
		return fmt.Errorf("create generation staging: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(staging)
		}
	}()
	if err := writeGeneration(staging, decision); err != nil {
		return err
	}
	final := filepath.Join(generations, decision.snapshot.Generation)
	if info, statErr := os.Stat(final); statErr == nil {
		if !info.IsDir() || !generationMatches(final, decision) {
			return errors.New("existing immutable generation disagrees with verified state")
		}
		if err := os.RemoveAll(staging); err != nil {
			return fmt.Errorf("remove redundant generation staging: %w", err)
		}
		committed = true
		return replaceCurrent(root, decision.snapshot.Generation)
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("inspect immutable generation: %w", statErr)
	}
	if err := os.Rename(staging, final); err != nil {
		return fmt.Errorf("publish immutable generation: %w", err)
	}
	committed = true
	if err := syncDirectory(generations); err != nil {
		return fmt.Errorf("sync generations directory: %w", err)
	}
	return replaceCurrent(root, decision.snapshot.Generation)
}

func generationMatches(directory string, decision candidateDecision) bool {
	epoch, err := readBoundedFile(filepath.Join(directory, "epoch.bin"), maximumEpochBytes)
	if err != nil || !bytes.Equal(epoch, decision.epochBytes) {
		return false
	}
	inputsDirectory := filepath.Join(directory, "inputs")
	entries, err := readBoundedDirectory(inputsDirectory, 64)
	if err != nil || len(entries) != len(decision.inputs) {
		return false
	}
	for index, expected := range decision.inputs {
		if entries[index].IsDir() || entries[index].Name() != fmt.Sprintf("%04d.bin", index) {
			return false
		}
		actual, readErr := readBoundedFile(filepath.Join(inputsDirectory, entries[index].Name()), maximumRecordBytes)
		if readErr != nil || !bytes.Equal(actual, expected) {
			return false
		}
	}
	return true
}

func writeGeneration(staging string, decision candidateDecision) error {
	inputsDirectory := filepath.Join(staging, "inputs")
	if err := os.Mkdir(inputsDirectory, 0o700); err != nil {
		return fmt.Errorf("create generation inputs: %w", err)
	}
	if err := writeSynced(filepath.Join(staging, "epoch.bin"), decision.epochBytes); err != nil {
		return err
	}
	for index, input := range decision.inputs {
		path := filepath.Join(inputsDirectory, fmt.Sprintf("%04d.bin", index))
		if err := writeSynced(path, input); err != nil {
			return err
		}
	}
	if err := syncDirectory(inputsDirectory); err != nil {
		return fmt.Errorf("sync generation inputs: %w", err)
	}
	if err := syncDirectory(staging); err != nil {
		return fmt.Errorf("sync generation: %w", err)
	}
	return nil
}

func writeSynced(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create immutable state file: %w", err)
	}
	if _, err = file.Write(contents); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("write immutable state file: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close immutable state file: %w", closeErr)
	}
	return nil
}

func replaceCurrent(root, generation string) error {
	temporary, err := os.CreateTemp(root, ".current-")
	if err != nil {
		return fmt.Errorf("create current pointer staging: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.WriteString(generation + "\n")
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return fmt.Errorf("write current pointer staging: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close current pointer staging: %w", closeErr)
	}
	if err := os.Rename(temporaryPath, filepath.Join(root, "current")); err != nil {
		return fmt.Errorf("replace current pointer: %w", err)
	}
	if err := syncDirectory(root); err != nil {
		return fmt.Errorf("sync current pointer: %w", err)
	}
	return nil
}
