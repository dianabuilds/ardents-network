package releasedecision

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Store is the durable floor store interface owned by the caller. The
// Release Decision Module publishes the verified consecutive root chain
// and the successor floors through CommitFloors, reads the previously
// committed floors through ReadFloors, and lets the caller serialize
// exclusive access through Close. An implementation must be fail-closed:
// any tamper, truncation, or missing file is reported as an error and
// never silently downgrades the security watermark.
type Store interface {
	// ReadFloors returns the previously committed floor set. An empty
	// FloorSet with a nil error means the store has never been committed.
	ReadFloors() (FloorSet, error)
	// CommitFloors atomically publishes the supplied floor set. After
	// CommitFloors returns nil, a fresh OpenFloorStore on the same path
	// must observe the same floors. CommitFloors must reject lower or
	// same-version/different-digest successor floors and must never
	// leave a partial successor on disk.
	CommitFloors(floors FloorSet) error
	// Close releases the exclusive lease and flushes any in-memory state.
	Close() error
}

// floorStore is the file-based owned state-root implementation. It stores
// each role's version + digest as a separate file under a generations
// directory and uses an atomic directory rename to publish successor
// generations, mirroring the network/store atomic publication pattern.
type floorStore struct {
	path      string
	closed    bool
	committed bool
}

// OpenFloorStore claims or verifies the owned state root at the supplied
// path and returns a Store ready to read and commit floor generations. The
// root is created if it does not exist and is rejected if it is a symlink,
// a non-directory, or contains unknown entries. A previously committed
// floor set is loaded synchronously; an invalid existing floor is reported
// as an error so a tampered state root fails closed at first use.
func OpenFloorStore(path string) (Store, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("releasedecision: resolve state root: %w", err)
	}
	if err := inspectFloorStoreRoot(absolute); err != nil {
		return nil, err
	}
	if err := prepareFloorStoreRoot(absolute); err != nil {
		return nil, err
	}
	return &floorStore{path: absolute}, nil
}

// floorStoreMarker is the single-byte marker that marks an owned state
// root. The marker is a regular file (never a symlink) and its contents
// are exactly the marker bytes followed by a newline.
const (
	floorStoreMarkerName = ".ardents-release-decision-v1"
	floorStoreMarker     = "ardents-release-decision-v1\n"
	floorStoreLockName   = ".ardents-release-decision-lock"
)

// inspectFloorStoreRoot refuses non-owned or symlinked state roots and
// refuses to claim a state root that already contains unknown entries.
func inspectFloorStoreRoot(root string) error {
	info, err := os.Lstat(root)
	if err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("releasedecision: state root is not an owned directory")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("releasedecision: inspect state root: %w", err)
	} else if err := os.MkdirAll(root, 0o700); err != nil {
		return fmt.Errorf("releasedecision: create state root: %w", err)
	}
	markerPath := filepath.Join(root, floorStoreMarkerName)
	markerInfo, markerErr := os.Lstat(markerPath)
	if markerErr == nil {
		if !markerInfo.Mode().IsRegular() || markerInfo.Mode()&os.ModeSymlink != 0 {
			return errors.New("releasedecision: state root marker is not a regular file")
		}
		return nil
	}
	if !os.IsNotExist(markerErr) {
		return fmt.Errorf("releasedecision: inspect state root marker: %w", markerErr)
	}
	entries, readErr := readFloorStoreDirectory(root, 2)
	if readErr != nil {
		return fmt.Errorf("releasedecision: inspect state root: %w", readErr)
	}
	if len(entries) > 1 {
		return errors.New("releasedecision: refusing to claim a non-empty unowned state root")
	}
	if len(entries) == 1 && entries[0].Name() != floorStoreLockName {
		return errors.New("releasedecision: refusing to claim a non-empty unowned state root")
	}
	return nil
}

// prepareFloorStoreRoot writes the ownership marker, prepares the
// generations directory, and removes any interrupted staging from a
// previous crash.
func prepareFloorStoreRoot(root string) error {
	if err := writeFloorStoreMarker(root); err != nil {
		return err
	}
	generations := filepath.Join(root, "generations")
	if err := os.MkdirAll(generations, 0o700); err != nil {
		return fmt.Errorf("releasedecision: create generations directory: %w", err)
	}
	return cleanupFloorStoreStaging(root, generations)
}
