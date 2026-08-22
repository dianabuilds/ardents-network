package updatetransaction

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type ownedStore struct {
	root string
	ops  durabilityOps
}

func acquireStore(root string, generation uint64) (*ownedStore, rootInspection, error) {
	var inspection rootInspection
	if err := validateOwnedPath(root); err != nil {
		return nil, inspection, errRecordInvalid
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, inspection, errors.Join(errRecordInvalid, err)
	}
	store := &ownedStore{root: root, ops: nativeDurability()}
	inspection, err = store.inspect(generation)
	if err != nil {
		return nil, inspection, err
	}
	return store, inspection, nil
}

func (store *ownedStore) inspect(generation uint64) (rootInspection, error) {
	var inspection rootInspection
	rootNames := []string{".ardents-update-transaction-lock", ".ardents-update-transaction-v1", "current", "generations", "staging", "transactions"}
	schemaPath := filepath.Join(store.root, "schema-current")
	if _, err := os.Lstat(schemaPath); err == nil {
		rootNames = append(rootNames, "schema-current")
	} else if !errors.Is(err, os.ErrNotExist) {
		return inspection, err
	}
	if err := requireNames(store.root, rootNames); err != nil {
		return inspection, err
	}
	marker, err := readExactFile(filepath.Join(store.root, ".ardents-update-transaction-v1"), len(rootMarker))
	if err != nil || string(marker) != rootMarker {
		return inspection, errors.Join(errRecordInvalid, err)
	}
	if err := requireNames(filepath.Join(store.root, "staging"), nil); err != nil {
		return inspection, err
	}
	currentRaw, err := readBoundedFile(filepath.Join(store.root, "current"), maximumRecordBytes)
	if err != nil {
		return inspection, err
	}
	selection, err := decodeCurrent(currentRaw)
	if err != nil || selection.Current.Generation != selection.Transaction {
		return inspection, errors.Join(errRecordInvalid, err)
	}
	if len(rootNames) == 7 {
		schemaRaw, readErr := readExactFile(schemaPath, recordHeaderBytes+schemaRecordBodyBytes)
		if readErr != nil {
			return inspection, errors.Join(errRecordInvalid, readErr)
		}
		schema, decodeErr := decodeSchemaCurrent(schemaRaw)
		if decodeErr != nil || schema.Transaction != selection.Transaction {
			return inspection, errors.Join(errRecordInvalid, decodeErr)
		}
		inspection.schemaCurrent = &schema
		inspection.schemaRaw = schemaRaw
	}
	currentView, currentArtifact, currentManifest, err := store.inspectPayload("generations", selection.Current)
	if err != nil {
		return inspection, err
	}
	generationNames := []string{strconv.FormatUint(selection.Current.Generation, 10)}
	if selection.Rollback != nil {
		if _, _, _, err := store.inspectPayload("generations", *selection.Rollback); err != nil {
			return inspection, err
		}
		generationNames = append(generationNames, strconv.FormatUint(selection.Rollback.Generation, 10))
	}
	if err := requireNames(filepath.Join(store.root, "generations"), generationNames); err != nil {
		return inspection, err
	}
	transactionNames := []string(nil)
	if _, err := os.Lstat(store.generationPath("transactions", generation)); err == nil {
		transactionNames = []string{strconv.FormatUint(generation, 10)}
	} else if !errors.Is(err, os.ErrNotExist) {
		return inspection, err
	}
	if err := requireNames(filepath.Join(store.root, "transactions"), transactionNames); err != nil {
		return inspection, err
	}
	inspection.selection = selection
	inspection.currentCustody = currentView.CustodyNotice
	inspection.predecessor = predecessorInspection{CurrentRecordDigest: sha256.Sum256(currentRaw),
		Current: selection.Current, Rollback: selection.Rollback,
		ArtifactObservation: currentArtifact, ManifestObservation: currentManifest}
	return inspection, nil
}

func (store *ownedStore) inspectPayload(kind string, tuple inspectedTuple) (manifestView, [32]byte, [32]byte, error) {
	var view manifestView
	var artifactDigest, manifestDigest [32]byte
	directory := store.generationPath(kind, tuple.Generation)
	if err := requireNames(directory, []string{"artifact", "manifest.bin"}); err != nil {
		return view, artifactDigest, manifestDigest, err
	}
	if tuple.Length > maximumArtifactBytes {
		return view, artifactDigest, manifestDigest, errRecordInvalid
	}
	artifact, err := readExactFile(filepath.Join(directory, "artifact"), int(tuple.Length))
	if err != nil {
		return view, artifactDigest, manifestDigest, err
	}
	manifest, err := readBoundedFile(filepath.Join(directory, "manifest.bin"), maximumRecordBytes)
	if err != nil {
		return view, artifactDigest, manifestDigest, err
	}
	artifactDigest, manifestDigest = sha256.Sum256(artifact), sha256.Sum256(manifest)
	view, err = decodeManifest(manifest)
	if err != nil || artifactDigest != tuple.Artifact || manifestDigest != tuple.Manifest ||
		view.Generation != tuple.Generation || view.Length != tuple.Length || view.Artifact != tuple.Artifact ||
		(tuple.Generation == 0 && hex.EncodeToString(manifestDigest[:]) != v0BootstrapManifestHex) {
		return manifestView{}, artifactDigest, manifestDigest, errors.Join(errRecordInvalid, err)
	}
	return view, artifactDigest, manifestDigest, nil
}

func (store *ownedStore) prepare(generation uint64) error {
	transaction := store.generationPath("transactions", generation)
	if err := os.Mkdir(transaction, 0o700); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(transaction, "journal"), 0o700); err != nil {
		return err
	}
	return store.ops.syncDirectory(filepath.Join(store.root, "transactions"))
}

func (store *ownedStore) stage(generation uint64, artifact, manifest []byte, operations stageOperations) error {
	temporary := stageTemporaryPath(store.root, generation)
	directory := store.generationPath("staging", generation)
	if err := os.Mkdir(temporary, 0o700); err != nil {
		return err
	}
	if err := writeStageFile(operations, filepath.Join(temporary, "artifact"), artifact); err != nil {
		return err
	}
	if err := writeStageFile(operations, filepath.Join(temporary, "manifest.bin"), manifest); err != nil {
		return err
	}
	artifactCheck, artifactErr := readExactFile(filepath.Join(temporary, "artifact"), len(artifact))
	manifestCheck, manifestErr := readExactFile(filepath.Join(temporary, "manifest.bin"), len(manifest))
	if artifactErr != nil || manifestErr != nil || sha256.Sum256(artifactCheck) != sha256.Sum256(artifact) ||
		sha256.Sum256(manifestCheck) != sha256.Sum256(manifest) {
		return errors.Join(errRecordInvalid, artifactErr, manifestErr)
	}
	if err := operations.renameDirectory(temporary, directory); err != nil {
		return err
	}
	if err := operations.acknowledge(filepath.Join(store.root, "staging")); err != nil {
		return fmt.Errorf("acknowledge staging parent: %w", err)
	}
	return nil
}

func (store *ownedStore) activate(generation uint64, selection currentSelection, expectedCurrent [32]byte, control *applyInterruptionControl) error {
	if selection.Rollback == nil {
		return errors.Join(errRecordInvalid, store.cleanup(generation))
	}
	if _, _, _, err := store.inspectPayload("staging", selection.Current); err != nil {
		return errors.Join(err, store.cleanup(generation))
	}
	if _, _, _, err := store.inspectPayload("generations", *selection.Rollback); err != nil {
		return errors.Join(err, store.cleanup(generation))
	}
	current, err := readBoundedFile(filepath.Join(store.root, "current"), maximumRecordBytes)
	if err != nil || sha256.Sum256(current) != expectedCurrent {
		return errors.Join(errRecordInvalid, err, store.cleanup(generation))
	}
	if err := applyCheckpoint(control, true, "publish-generation"); err != nil {
		return err
	}
	if err := store.ops.publishGeneration(store.generationPath("staging", generation),
		filepath.Join(store.root, "generations", strconv.FormatUint(generation, 10))); err != nil {
		return err
	}
	if err := applyCheckpoint(control, false, "publish-generation"); err != nil {
		return err
	}
	raw, err := encodeCurrent(selection)
	if err != nil {
		return err
	}
	var token [8]byte
	if _, err := io.ReadFull(rand.Reader, token[:]); err != nil {
		return err
	}
	temporary := filepath.Join(store.root, ".current."+hex.EncodeToString(token[:])+".tmp")
	if err := applyCheckpoint(control, true, "current-temp"); err != nil {
		return err
	}
	if err := writeNewFile(temporary, raw); err != nil {
		return err
	}
	if err := applyCheckpoint(control, false, "current-temp"); err != nil {
		return err
	}
	if err := applyCheckpoint(control, true, "replace-current"); err != nil {
		return err
	}
	if err := store.ops.replaceCurrent(temporary, filepath.Join(store.root, "current")); err != nil {
		return err
	}
	if err := applyCheckpoint(control, false, "replace-current"); err != nil {
		return err
	}
	if err := applyCheckpoint(control, true, "durability-ack"); err != nil {
		return err
	}
	if err := store.ops.syncDirectory(store.root); err != nil {
		return err
	}
	if err := applyCheckpoint(control, false, "durability-ack"); err != nil {
		return err
	}
	return nil
}

func (store *ownedStore) release() error {
	return store.ops.syncDirectory(store.root)
}

func (store *ownedStore) cleanup(generation uint64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	staging := store.generationPath("staging", generation)
	temporary := stageTemporaryPath(store.root, generation)
	transaction := store.generationPath("transactions", generation)
	paths := []string{filepath.Join(staging, "artifact"), filepath.Join(staging, "manifest.bin"), staging,
		filepath.Join(temporary, "artifact"), filepath.Join(temporary, "manifest.bin"), temporary}
	var result error
	for state := stateReleaseAccepted; state <= stateRepairRequired; state++ {
		name, err := journalFileName(state)
		result = errors.Join(result, err)
		paths = append(paths, filepath.Join(transaction, "journal", name))
	}
	paths = append(paths, filepath.Join(transaction, "journal"), transaction)
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return errors.Join(result, err)
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			result = errors.Join(result, err)
		}
	}
	return errors.Join(result, boundedCleanup(ctx, store.ops.syncDirectory, filepath.Join(store.root, "staging"), filepath.Join(store.root, "transactions")))
}

func boundedCleanup(ctx context.Context, operation func(string) error, paths ...string) error {
	var result error
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return errors.Join(result, err)
		}
		result = errors.Join(result, operation(path))
		if err := ctx.Err(); err != nil {
			return errors.Join(result, err)
		}
	}
	return result
}

func (store *ownedStore) generationPath(kind string, generation uint64) string {
	return filepath.Join(store.root, kind, strconv.FormatUint(generation, 10))
}

func requireNames(path string, expected []string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || validateOwnedEntry(path) != nil {
		return errors.Join(errRecordInvalid, err)
	}
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != len(expected) {
		return errors.Join(errRecordInvalid, err)
	}
	want := make(map[string]bool, len(expected))
	for _, name := range expected {
		want[name] = true
	}
	for _, entry := range entries {
		if !want[entry.Name()] {
			return errRecordInvalid
		}
		// The permanent lock is already validated through the exact held
		// OS handle by acquireOwnedLock. Reopening it here would violate the
		// one-open invariant and fails by design under Windows zero sharing.
		if entry.Name() == lockFileName {
			continue
		}
		if validateOwnedEntry(filepath.Join(path, entry.Name())) != nil {
			return errRecordInvalid
		}
	}
	return nil
}
