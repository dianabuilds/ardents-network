package release

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// floorPersistence is the durable floor store implementation seam owned by release.
// Release Decision Module publishes the verified consecutive root chain
// and the successor floors through CommitFloors, reads the previously
// committed floors through ReadFloors, and lets the caller serialize
// exclusive access through Close. An implementation must be fail-closed:
// any tamper, truncation, or missing file is reported as an error and
// never silently downgrades the security watermark.
type floorPersistence interface {
	// ReadFloors returns the committed set. Root-only means metadata has not
	// yet been accepted; an empty set means the store has never been committed.
	ReadFloors() (FloorSet, error)
	// CommitRoot durably publishes one fully verified root before the
	// verifier uses that root to authorize any later root or metadata.
	// rootChain ends at the supplied root and contains the exact bytes
	// verified during this evaluation.
	CommitRoot(version int64, digest []byte, rootChain [][]byte) error
	// CommitFloors atomically publishes the supplied floor set and exact
	// verified consecutive root bytes. After
	// CommitFloors returns nil, a fresh release open on the same path
	// must observe the same floors. CommitFloors must reject lower or
	// same-version/different-digest successor floors and must never
	// leave a partial successor on disk.
	CommitFloors(floors FloorSet, rootChain [][]byte) error
	// Close releases the exclusive lease and flushes any in-memory state.
	Close() error
}

// floorStore is the file-based owned state-root implementation. It stores
// each role's version + digest as a separate file under a generations
// directory and uses an atomic directory rename to publish successor
// generations, mirroring the network/store atomic publication pattern.
type floorStore struct {
	path   string
	lock   *os.File
	closed bool
}

// Verifier owns one exclusive, durable release-floor state root. It verifies
// offline inputs against that state and releases its lease on Close.
type Verifier struct {
	store floorPersistence
}

// Open claims or verifies the owned state root at the supplied path and
// returns the sole public release verifier. The root is created if it does
// not exist and is rejected if it is a symlink, a non-directory, or contains
// unknown entries. A previously committed floor set is loaded synchronously;
// an invalid existing floor fails closed before evaluation.
func Open(path string) (*Verifier, error) {
	store, err := openFloorStore(path)
	if err != nil {
		return nil, err
	}
	return &Verifier{store: store}, nil
}

// Close releases the verifier's exclusive state-root lease. A nil verifier is
// invalid and reports an error rather than silently succeeding.
func (verifier *Verifier) Close() error {
	if verifier == nil || verifier.store == nil {
		return errors.New("release: verifier is nil")
	}
	return verifier.store.Close()
}

// openFloorStore claims or verifies the owned state root at the supplied path
// and returns an internal store ready to read and commit floor generations. The
// root is created if it does not exist and is rejected if it is a symlink,
// a non-directory, or contains unknown entries. A previously committed
// floor set is loaded synchronously; an invalid existing floor is reported
// as an error so a tampered state root fails closed at first use.
func openFloorStore(path string) (floorPersistence, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("release: resolve state root: %w", err)
	}
	if err := inspectFloorStoreRoot(absolute); err != nil {
		return nil, err
	}
	lock, err := acquireFloorStoreLease(absolute)
	if err != nil {
		return nil, err
	}
	store := &floorStore{path: absolute, lock: lock}
	if err := prepareFloorStoreRoot(absolute); err != nil {
		return nil, errors.Join(err, store.Close())
	}
	if _, err := store.ReadFloors(); err != nil {
		return nil, errors.Join(fmt.Errorf("release: validate existing state: %w", err), store.Close())
	}
	return store, nil
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
			return errors.New("release: state root is not an owned directory")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("release: inspect state root: %w", err)
	} else if err := os.MkdirAll(root, 0o700); err != nil {
		return fmt.Errorf("release: create state root: %w", err)
	}
	markerPath := filepath.Join(root, floorStoreMarkerName)
	markerInfo, markerErr := os.Lstat(markerPath)
	if markerErr == nil {
		if !markerInfo.Mode().IsRegular() || markerInfo.Mode()&os.ModeSymlink != 0 {
			return errors.New("release: state root marker is not a regular file")
		}
		entries, readErr := readFloorStoreDirectory(root, 70)
		if readErr != nil {
			return fmt.Errorf("release: inspect owned state root: %w", readErr)
		}
		for _, entry := range entries {
			name := entry.Name()
			if name == floorStoreMarkerName || name == floorStoreLockName || name == "generations" ||
				name == "current" || strings.HasPrefix(name, ".current-") {
				continue
			}
			return fmt.Errorf("release: unknown state-root entry %q", name)
		}
		return nil
	}
	if !os.IsNotExist(markerErr) {
		return fmt.Errorf("release: inspect state root marker: %w", markerErr)
	}
	entries, readErr := readFloorStoreDirectory(root, 2)
	if readErr != nil {
		return fmt.Errorf("release: inspect state root: %w", readErr)
	}
	if len(entries) > 1 {
		return errors.New("release: refusing to claim a non-empty unowned state root")
	}
	if len(entries) == 1 && entries[0].Name() != floorStoreLockName {
		return errors.New("release: refusing to claim a non-empty unowned state root")
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
		return fmt.Errorf("release: create generations directory: %w", err)
	}
	return cleanupFloorStoreStaging(root, generations)
}
