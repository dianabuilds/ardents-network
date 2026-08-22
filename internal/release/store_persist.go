package release

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
var floorGenerationName = regexp.MustCompile(`^[0-9a-f]{64}$`)

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
	raw, err := readBoundedFloorFile(pointerPath, 80)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return FloorSet{}, nil
		}
		return FloorSet{}, err
	}
	name := strings.TrimRight(string(raw), "\n")
	if string(raw) != name+"\n" || !floorGenerationName.MatchString(name) {
		return FloorSet{}, errors.New("release: state root pointer is invalid")
	}
	return readFloorGeneration(store.path, name)
}

// CommitFloors atomically publishes the supplied floor set as a single
// immutable generation, then points the state root at it.
func (store *floorStore) CommitFloors(floors FloorSet, rootChain [][]byte) error {
	if err := store.available(); err != nil {
		return err
	}
	if err := validateFloorSet(floors); err != nil {
		return err
	}
	existing, err := store.ReadFloors()
	if err != nil {
		return fmt.Errorf("release: read existing floors: %w", err)
	}
	if err := assertFloorAdvance(existing, floors); err != nil {
		return err
	}
	archive, err := validateRootArchive(rootChain, floors)
	if err != nil {
		return err
	}
	payload, err := encodeFloorGeneration(floors)
	if err != nil {
		return err
	}
	name := floorGenerationID(payload, archive)
	generations := filepath.Join(store.path, "generations")
	final := filepath.Join(generations, name)
	if existing, err := os.Stat(final); err == nil {
		if !existing.IsDir() {
			return errors.New("release: existing generation entry is not a directory")
		}
		stored, loadErr := readFloorGenerationFromPath(final)
		if loadErr != nil || !floorSetEqual(stored, floors) {
			return errors.New("release: existing generation disagrees with supplied floors")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("release: inspect generation: %w", err)
	} else {
		if err := publishFloorGeneration(generations, name, payload, archive); err != nil {
			return err
		}
	}
	if err := writeFloorStorePointer(store.path, name); err != nil {
		return err
	}
	return nil
}

// CommitRoot publishes the next trusted root while preserving any older
// timestamp, snapshot, and targets floors. This is a separate durable step:
// callers must complete it before relying on the root for later verification.
func (store *floorStore) CommitRoot(version int64, digest []byte, rootChain [][]byte) error {
	existing, err := store.ReadFloors()
	if err != nil {
		return err
	}
	next := existing
	next.RootVersion = version
	next.RootDigest = append([]byte(nil), digest...)
	return store.CommitFloors(next, rootChain)
}

// Close releases the in-process marker.
func (store *floorStore) Close() error {
	if store.closed {
		return nil
	}
	store.closed = true
	return store.releaseLease()
}

// available reports a stored error if the store has been closed.
func (store *floorStore) available() error {
	if store.closed {
		return errors.New("release: state root is closed")
	}
	return nil
}

// validateFloorSet accepts a root-only generation because each verified root
// must become durable before timestamp verification. Metadata floors, once
// present, must always be complete as one atomic group.
func validateFloorSet(floors FloorSet) error {
	if floors.RootVersion <= 0 || len(floors.RootDigest) != 32 {
		return errors.New("release: root floor is incomplete")
	}
	emptyMetadata := floors.TimestampVersion == 0 && len(floors.TimestampDigest) == 0 &&
		floors.SnapshotVersion == 0 && len(floors.SnapshotDigest) == 0 &&
		floors.TargetsVersion == 0 && len(floors.TargetsDigest) == 0
	if emptyMetadata {
		return nil
	}
	if floors.TimestampVersion <= 0 || len(floors.TimestampDigest) != 32 ||
		floors.SnapshotVersion <= 0 || len(floors.SnapshotDigest) != 32 ||
		floors.TargetsVersion <= 0 || len(floors.TargetsDigest) != 32 {
		return errors.New("release: metadata floors are incomplete")
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
		return errors.New("release: root version is lower than the durable floor")
	}
	if next.RootVersion == previous.RootVersion && !bytes.Equal(next.RootDigest, previous.RootDigest) {
		return errors.New("release: root digest differs from the durable floor at the same version")
	}
	if next.TimestampVersion < previous.TimestampVersion {
		return errors.New("release: timestamp version is lower than the durable floor")
	}
	if next.TimestampVersion == previous.TimestampVersion && !bytes.Equal(next.TimestampDigest, previous.TimestampDigest) {
		return errors.New("release: timestamp digest differs from the durable floor at the same version")
	}
	if next.SnapshotVersion < previous.SnapshotVersion {
		return errors.New("release: snapshot version is lower than the durable floor")
	}
	if next.SnapshotVersion == previous.SnapshotVersion && !bytes.Equal(next.SnapshotDigest, previous.SnapshotDigest) {
		return errors.New("release: snapshot digest differs from the durable floor at the same version")
	}
	if next.TargetsVersion < previous.TargetsVersion {
		return errors.New("release: targets version is lower than the durable floor")
	}
	if next.TargetsVersion == previous.TargetsVersion && !bytes.Equal(next.TargetsDigest, previous.TargetsDigest) {
		return errors.New("release: targets digest differs from the durable floor at the same version")
	}
	return nil
}
