package releasedecision

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// floorDigestPrefix is the byte prefix written before each role's digest
// in a generation state.bin file. The format is "role version=
// digest=<hex>\n" per role, in canonical order.
const floorDigestPrefix = "role="

// floorGenerationName matches the canonical generation name pattern:
// eight hex characters derived from the SHA-256 of the generation
// state.bin. The store never invents other names.
var floorGenerationName = regexp.MustCompile(`^[0-9a-f]{8}$`)

// floorRoles is the canonical ordered list of role names in a generation.
// The order is fixed so commit and read agree byte-for-byte.
var floorRoles = [...]string{"root", "timestamp", "snapshot", "targets"}

// ReadFloors returns the previously committed floor set, or an empty
// FloorSet when the state root has never been committed.
func (store *floorStore) ReadFloors() (FloorSet, error) {
	if err := store.available(); err != nil {
		return FloorSet{}, err
	}
	pointerPath := filepath.Join(store.path, "current")
	raw, err := readBoundedFloorFile(pointerPath, 16)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return FloorSet{}, nil
		}
		return FloorSet{}, err
	}
	name := strings.TrimRight(string(raw), "\n")
	if string(raw) != name+"\n" || !floorGenerationName.MatchString(name) {
		return FloorSet{}, errors.New("releasedecision: state root pointer is invalid")
	}
	return readFloorGeneration(store.path, name)
}

// CommitFloors atomically publishes the supplied floor set as a single
// immutable generation, then points the state root at it.
func (store *floorStore) CommitFloors(floors FloorSet) error {
	if err := store.available(); err != nil {
		return err
	}
	if err := validateFloorSet(floors); err != nil {
		return err
	}
	existing, err := store.ReadFloors()
	if err != nil {
		return fmt.Errorf("releasedecision: read existing floors: %w", err)
	}
	if err := assertFloorAdvance(existing, floors); err != nil {
		return err
	}
	payload, err := encodeFloorGeneration(floors)
	if err != nil {
		return err
	}
	name := floorGenerationID(payload)
	generations := filepath.Join(store.path, "generations")
	final := filepath.Join(generations, name)
	if existing, err := os.Stat(final); err == nil {
		if !existing.IsDir() {
			return errors.New("releasedecision: existing generation entry is not a directory")
		}
		stored, loadErr := readFloorGenerationFromPath(final)
		if loadErr != nil || !floorSetEqual(stored, floors) {
			return errors.New("releasedecision: existing generation disagrees with supplied floors")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("releasedecision: inspect generation: %w", err)
	} else {
		if err := publishFloorGeneration(generations, name, payload); err != nil {
			return err
		}
	}
	if err := writeFloorStorePointer(store.path, name); err != nil {
		return err
	}
	store.committed = true
	return nil
}

// Close releases the in-process marker.
func (store *floorStore) Close() error {
	store.closed = true
	return nil
}

// available reports a stored error if the store has been closed.
func (store *floorStore) available() error {
	if store.closed {
		return errors.New("releasedecision: state root is closed")
	}
	return nil
}

// validateFloorSet rejects zero or partial floor sets.
func validateFloorSet(floors FloorSet) error {
	if floors.RootVersion <= 0 || len(floors.RootDigest) != 32 {
		return errors.New("releasedecision: root floor is incomplete")
	}
	if floors.TimestampVersion <= 0 || len(floors.TimestampDigest) != 32 {
		return errors.New("releasedecision: timestamp floor is incomplete")
	}
	if floors.SnapshotVersion <= 0 || len(floors.SnapshotDigest) != 32 {
		return errors.New("releasedecision: snapshot floor is incomplete")
	}
	if floors.TargetsVersion <= 0 || len(floors.TargetsDigest) != 32 {
		return errors.New("releasedecision: targets floor is incomplete")
	}
	return nil
}

// assertFloorAdvance rejects same-version/different-digest and any
// lower successor version. It is the watermarking invariant.
func assertFloorAdvance(previous, next FloorSet) error {
	if previous.RootVersion == 0 {
		return nil
	}
	if next.RootVersion < previous.RootVersion {
		return errors.New("releasedecision: root version is lower than the durable floor")
	}
	if next.RootVersion == previous.RootVersion && !bytes.Equal(next.RootDigest, previous.RootDigest) {
		return errors.New("releasedecision: root digest differs from the durable floor at the same version")
	}
	if next.TimestampVersion < previous.TimestampVersion {
		return errors.New("releasedecision: timestamp version is lower than the durable floor")
	}
	if next.TimestampVersion == previous.TimestampVersion && !bytes.Equal(next.TimestampDigest, previous.TimestampDigest) {
		return errors.New("releasedecision: timestamp digest differs from the durable floor at the same version")
	}
	if next.SnapshotVersion < previous.SnapshotVersion {
		return errors.New("releasedecision: snapshot version is lower than the durable floor")
	}
	if next.SnapshotVersion == previous.SnapshotVersion && !bytes.Equal(next.SnapshotDigest, previous.SnapshotDigest) {
		return errors.New("releasedecision: snapshot digest differs from the durable floor at the same version")
	}
	if next.TargetsVersion < previous.TargetsVersion {
		return errors.New("releasedecision: targets version is lower than the durable floor")
	}
	if next.TargetsVersion == previous.TargetsVersion && !bytes.Equal(next.TargetsDigest, previous.TargetsDigest) {
		return errors.New("releasedecision: targets digest differs from the durable floor at the same version")
	}
	return nil
}
