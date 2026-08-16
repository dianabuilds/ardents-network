package blockedverify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const maximumRegistryEntries = 4096

type replayRegistry struct {
	Schema  string        `json:"schema"`
	Entries []replayEntry `json:"entries"`
}

type replayEntry struct {
	RunIDHash    string `json:"run_id_hash"`
	NonceHash    string `json:"nonce_hash"`
	ManifestHash string `json:"manifest_hash"`
	DecisionHash string `json:"decision_hash"`
	State        string `json:"state"`
}

type replayTransaction struct {
	registry, statePath string
	state               replayRegistry
	index               int
	lease               *registryLease
}

func beginRun(registryRoot, runID, nonce, manifestHash, decisionHash, bundleRoot, outputPath string) (
	*replayTransaction, bool, string,
) {
	registry, registryErr := filepath.Abs(registryRoot)
	bundle, bundleErr := filepath.Abs(bundleRoot)
	if registryErr != nil || bundleErr != nil || registry == bundle || withinPath(bundle, registry) || withinPath(registry, bundle) {
		return nil, false, "verifier replay registry is missing or overlaps the evidence bundle"
	}
	aliased, err := pathHasSymlink(filepath.Dir(registry))
	if err != nil || aliased || ensureRegistryDirectory(registry) != nil {
		return nil, false, "verifier replay registry is unavailable or aliased"
	}
	lease, err := acquireRegistryLock(filepath.Join(registry, ".consume-lock"))
	if err != nil {
		return nil, false, "verifier replay registry is busy or unavailable"
	}
	statePath := filepath.Join(registry, "consumed.json")
	state, reason := readRegistry(statePath)
	if reason != "" {
		_ = lease.close()
		return nil, false, reason
	}
	runDigest := sha256.Sum256([]byte(runID))
	runHash := hex.EncodeToString(runDigest[:])
	for index, entry := range state.Entries {
		if entry.RunIDHash != runHash && entry.NonceHash != nonce {
			continue
		}
		if entry.RunIDHash == runHash && entry.NonceHash == nonce && entry.ManifestHash == manifestHash &&
			entry.State == "pending" {
			transaction := &replayTransaction{registry, statePath, state, index, lease}
			if err := removePendingVerdictTemporary(outputPath); err != nil {
				transaction.abandon()
				return nil, false, "pending replay verdict temporary could not be recovered"
			}
			_, outputErr := os.Lstat(outputPath)
			if outputErr == nil {
				return transaction, true, ""
			}
			if entry.DecisionHash != decisionHash {
				transaction.abandon()
				return nil, false, "pending replay decision differs from the original reservation"
			}
			return transaction, false, ""
		}
		_ = lease.close()
		return nil, false, "manifest nonce or run identity has already been consumed"
	}
	if len(state.Entries) >= maximumRegistryEntries {
		_ = lease.close()
		return nil, false, "verifier replay registry reached its bounded capacity"
	}
	state.Entries = append(state.Entries, replayEntry{runHash, nonce, manifestHash, decisionHash, "pending"})
	index := len(state.Entries) - 1
	if err := replaceRegistryState(registry, statePath, state); err != nil {
		_ = lease.close()
		return nil, false, "verifier replay reservation could not be committed"
	}
	return &replayTransaction{registry, statePath, state, index, lease}, false, ""
}

func removePendingVerdictTemporary(outputPath string) error {
	temporary := outputPath + ".tmp"
	if err := os.Remove(temporary); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return syncDirectory(filepath.Dir(outputPath))
}

func (transaction *replayTransaction) decisionHash() string {
	return transaction.state.Entries[transaction.index].DecisionHash
}

func (transaction *replayTransaction) commit() error {
	transaction.state.Entries[transaction.index].State = "committed"
	err := replaceRegistryState(transaction.registry, transaction.statePath, transaction.state)
	return errors.Join(err, transaction.lease.close())
}

func (transaction *replayTransaction) abandon() { _ = transaction.lease.close() }

func ensureRegistryDirectory(path string) error {
	if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.Join(err, errors.New("registry is not an owned directory"))
	}
	return protectRegistryTree(path)
}

func readRegistry(path string) (replayRegistry, string) {
	state := replayRegistry{Schema: "ardents-h3-replay-registry-v1"}
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return state, ""
	}
	if _, err := decodeStrict(path, &state); err != nil || state.Schema != "ardents-h3-replay-registry-v1" ||
		len(state.Entries) > maximumRegistryEntries {
		return replayRegistry{}, "verifier replay registry state is invalid"
	}
	for _, entry := range state.Entries {
		if entry.State != "pending" && entry.State != "committed" {
			return replayRegistry{}, "verifier replay registry entry state is invalid"
		}
	}
	return state, ""
}

func replaceRegistryState(root, target string, state replayRegistry) error {
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	temporary := filepath.Join(root, ".consumed-next")
	if err := os.Remove(temporary); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := syncDirectory(root); err != nil {
		return err
	}
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(append(raw, '\n'))
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(temporary)
		return errors.Join(writeErr, syncErr, closeErr)
	}
	if err := protectRegistryTree(root); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := replaceRegistryFile(temporary, target); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := protectRegistryTree(root); err != nil {
		return err
	}
	return syncDirectory(root)
}

func withinPath(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != "." && relative != "" && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
