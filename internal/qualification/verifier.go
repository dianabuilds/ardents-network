package qualification

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var canonicalGeneration = regexp.MustCompile(`^[0-9a-f]{64}$`)

// OfflineCase identifies one persisted candidate generation and independent inputs.
type OfflineCase struct {
	Root             string
	NetworkID        [32]byte
	Authorities      map[[32]byte]ed25519.PublicKey
	Threshold        int
	Now              time.Time
	Materializations [][]byte
}

// Result is the terminal machine outcome of one offline verification.
type Result struct {
	Verdict    string `json:"verdict"`
	Reason     string `json:"reason"`
	Generation string `json:"generation,omitempty"`
	Epoch      uint64 `json:"epoch,omitempty"`
}

type offlineEvidence struct {
	generation string
	epoch      []byte
	inputs     [][]byte
}

// VerifyOffline independently recomputes one persisted candidate decision.
func VerifyOffline(input OfflineCase) Result {
	if err := validateCase(input); err != nil {
		return Result{Verdict: "invalid", Reason: err.Error()}
	}
	evidence, err := readEvidence(input.Root)
	if err != nil {
		return Result{Verdict: "invalid", Reason: err.Error()}
	}
	epoch, err := verifyOfflineChain(input, evidence.generation)
	if err != nil {
		return Result{Verdict: "fail", Reason: err.Error(), Generation: evidence.generation}
	}
	if err := verifyOfflineEvidence(input, evidence, epoch); err != nil {
		return Result{Verdict: "fail", Reason: err.Error(), Generation: evidence.generation}
	}
	return Result{
		Verdict: "pass", Reason: "independent offline verification passed",
		Generation: evidence.generation, Epoch: epoch.number,
	}
}

func validateCase(input OfflineCase) error {
	if input.Root == "" {
		return errors.New("evidence root is required")
	}
	if input.Threshold < 1 || input.Threshold > len(input.Authorities) {
		return errors.New("authority threshold is outside the authority set")
	}
	if len(input.Authorities) > 16 || len(input.Materializations) > 64 {
		return errors.New("qualification input exceeds its finite set bounds")
	}
	if input.Now.IsZero() {
		return errors.New("verification time is required")
	}
	for id, public := range input.Authorities {
		if len(public) != ed25519.PublicKeySize {
			return errors.New("authority public key has invalid length")
		}
		if keyID(public) != id {
			return errors.New("authority identifier does not match its public key")
		}
	}
	return nil
}

func readEvidence(root string) (offlineEvidence, error) {
	pointer, err := independentlyReadBounded(filepath.Join(root, "current"), 65)
	if err != nil {
		return offlineEvidence{}, fmt.Errorf("read candidate current pointer: %w", err)
	}
	generation := strings.TrimSuffix(string(pointer), "\n")
	if string(pointer) != generation+"\n" || !canonicalGeneration.MatchString(generation) {
		return offlineEvidence{}, errors.New("candidate current pointer is not canonical")
	}
	directory := filepath.Join(root, "generations", generation)
	epochBytes, err := independentlyReadBounded(filepath.Join(directory, "epoch.bin"), 1<<20)
	if err != nil {
		return offlineEvidence{}, fmt.Errorf("read candidate epoch: %w", err)
	}
	epoch, err := parseOfflineEpoch(epochBytes)
	if err != nil {
		return offlineEvidence{}, fmt.Errorf("frame candidate epoch: %w", err)
	}
	inputsDirectory := filepath.Join(directory, "inputs")
	entries, err := independentlyReadDirectory(inputsDirectory, 64)
	if err != nil {
		return offlineEvidence{}, fmt.Errorf("read candidate inputs: %w", err)
	}
	if len(entries) != int(epoch.cutoff) {
		return offlineEvidence{}, errors.New("candidate input file count does not match its cutoff")
	}
	inputs := make([][]byte, epoch.cutoff)
	for index := range inputs {
		path := filepath.Join(inputsDirectory, fmt.Sprintf("%04d.bin", index))
		inputs[index], err = independentlyReadBounded(path, 32<<10)
		if err != nil {
			return offlineEvidence{}, fmt.Errorf("read candidate input %d: %w", index, err)
		}
	}
	return offlineEvidence{generation: generation, epoch: epochBytes, inputs: inputs}, nil
}

func independentlyReadBounded(path string, maximum int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	contents, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if int64(len(contents)) > maximum {
		return nil, errors.New("candidate file exceeds its framing bound")
	}
	return contents, nil
}

func independentlyReadDirectory(path string, maximum int) ([]os.DirEntry, error) {
	directory, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	entries, readErr := directory.ReadDir(maximum + 1)
	closeErr := directory.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(entries) > maximum {
		return nil, errors.New("candidate directory exceeds its entry bound")
	}
	return entries, nil
}

func verifyOfflineEvidence(input OfflineCase, evidence offlineEvidence, epoch offlineEpoch) error {
	if fmt.Sprintf("%x", epoch.digest) != evidence.generation {
		return errors.New("generation name does not match the epoch digest")
	}
	if recordRoot(evidence.inputs, 0x10) != epoch.inputRoot {
		return errors.New("candidate input root is inconsistent")
	}
	accepted, rejected := independentlyEvaluate(input, epoch, evidence.inputs)
	if err := independentlyVerifyView(epoch, accepted, rejected); err != nil {
		return err
	}
	if err := verifyOfflineMaterials(epoch, accepted, input.Materializations); err != nil {
		return err
	}
	return nil
}

func verifyOfflineChain(input OfflineCase, current string) (offlineEpoch, error) {
	seen := make(map[string]bool)
	var load func(string, bool) (offlineEpoch, error)
	load = func(name string, tip bool) (offlineEpoch, error) {
		if !canonicalGeneration.MatchString(name) || seen[name] || len(seen) >= 64 {
			return offlineEpoch{}, errors.New("offline generation chain is cyclic or exceeds its bound")
		}
		seen[name] = true
		raw, err := independentlyReadBounded(filepath.Join(input.Root, "generations", name, "epoch.bin"), 1<<20)
		if err != nil {
			return offlineEpoch{}, fmt.Errorf("read offline chain epoch: %w", err)
		}
		epoch, err := verifyOfflineEpoch(input, raw, !tip)
		if err != nil || fmt.Sprintf("%x", epoch.digest) != name {
			return offlineEpoch{}, errors.Join(errors.New("offline generation chain member is invalid"), err)
		}
		if epoch.number == 1 {
			if epoch.previous != [32]byte{} {
				return offlineEpoch{}, errors.New("offline genesis previous digest is not zero")
			}
			return epoch, nil
		}
		prior, err := load(fmt.Sprintf("%x", epoch.previous), false)
		if err != nil || prior.number+1 != epoch.number || prior.digest != epoch.previous {
			return offlineEpoch{}, errors.Join(errors.New("offline epoch transition is invalid"), err)
		}
		return epoch, nil
	}
	return load(current, true)
}
