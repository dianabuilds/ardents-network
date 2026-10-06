package release

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

const (
	floorStoreMarkerName    = ".ardents-release-decision-v1"
	floorStoreMarker        = "ardents-release-decision-v1\n"
	floorStoreLockName      = ".ardents-release-decision-lock"
	floorStoreLeaseContents = "ardents-release-decision-exclusive-lease-v1\n"
	floorDigestPrefix       = "role="
)

var floorRoles = [...]string{"root", "timestamp", "snapshot", "targets"}
var floorGenerationName = regexp.MustCompile(`^[0-9a-f]{64}$`)
var stagedGenerationName = regexp.MustCompile(`^\.stage-[0-9]{1,10}$`)
var stagedPointerName = regexp.MustCompile(`^\.current-[0-9]{1,10}$`)

type floorStore struct {
	path    string
	lease   historyLease
	closed  bool
	failure error
	flush   func(string) error
}

func openFloorStore(path string) (result *floorStore, resultErr error) {
	return openFloorStoreMode(path, false)
}

func openFloorStoreMode(path string, retained bool) (result *floorStore, resultErr error) {
	if !historyPlatformSupported {
		return nil, errors.New("release: native history unavailable")
	}
	root, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if !retained {
		if err = os.MkdirAll(root, 0700); err != nil {
			return nil, err
		}
	}
	info, err := os.Lstat(root)
	if err != nil {
		if retained && errors.Is(err, os.ErrNotExist) {
			return nil, ErrTrustUnavailable
		}
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("release: invalid history root")
	}
	if err = checkHistoryRoot(info); err != nil {
		return nil, err
	}
	entries, err := readFloorStoreDirectory(root, 70)
	if err != nil {
		return nil, err
	}
	hasMarker := false
	for _, e := range entries {
		if e.Name() == floorStoreMarkerName {
			hasMarker = true
		}
		switch e.Name() {
		case floorStoreMarkerName, floorStoreLockName, "generations", "current":
		default:
			if !stagedPointerName.MatchString(e.Name()) {
				return nil, errors.New("release: foreign history entry")
			}
		}
	}
	if retained && !hasMarker {
		return nil, ErrTrustUnavailable
	}
	if !hasMarker && (len(entries) > 1 || len(entries) == 1 && entries[0].Name() != floorStoreLockName) {
		return nil, errors.New("release: nonempty unowned history")
	}
	lockPath := filepath.Join(root, floorStoreLockName)
	if _, err = os.Lstat(lockPath); errors.Is(err, os.ErrNotExist) {
		if hasMarker {
			return nil, errors.New("release: retained history lease file missing")
		}
		if err = writeSyncedFile(lockPath, []byte(floorStoreLeaseContents)); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	contents, err := readBoundedFloorFile(lockPath, int64(len(floorStoreLeaseContents)))
	if err != nil || string(contents) != floorStoreLeaseContents {
		return nil, errors.New("release: invalid history lease")
	}
	lease, err := acquireHistoryLease(root)
	if err != nil {
		return nil, err
	}
	s := &floorStore{path: root, lease: lease, flush: syncDirectory}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, s.Close())
		}
	}()
	if hasMarker {
		marker, err := readBoundedFloorFile(filepath.Join(root, floorStoreMarkerName), int64(len(floorStoreMarker)))
		if err != nil || string(marker) != floorStoreMarker {
			return nil, errors.New("release: invalid history marker")
		}
	} else {
		if err = writeSyncedFile(filepath.Join(root, floorStoreMarkerName), []byte(floorStoreMarker)); err != nil {
			return nil, err
		}
		if err = os.Mkdir(filepath.Join(root, "generations"), 0700); err != nil {
			return nil, err
		}
		if err = syncDirectory(root); err != nil {
			return nil, err
		}
	}
	floors, err := s.ReadFloors()
	if err != nil {
		return nil, err
	}
	if retained && !completeFloors(floors) {
		return nil, ErrTrustUnavailable
	}
	if err = s.recoverWriterResidue(); err != nil {
		return nil, err
	}
	if err = s.confirmDurability(); err != nil {
		return nil, err
	}
	return s, nil
}

func completeFloors(f FloorSet) bool {
	return f.RootVersion > 0 && len(f.RootDigest) == sha256.Size &&
		f.TimestampVersion > 0 && len(f.TimestampDigest) == sha256.Size &&
		f.SnapshotVersion > 0 && len(f.SnapshotDigest) == sha256.Size &&
		f.TargetsVersion > 0 && len(f.TargetsDigest) == sha256.Size
}

func (s *floorStore) ReadFloors() (FloorSet, error) {
	if s.closed {
		return FloorSet{}, ErrClosed
	}
	if s.failure != nil {
		return FloorSet{}, s.failure
	}
	entries, err := readFloorStoreDirectory(filepath.Join(s.path, "generations"), 64)
	if err != nil {
		return FloorSet{}, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || (!floorGenerationName.MatchString(entry.Name()) && !stagedGenerationName.MatchString(entry.Name())) {
			return FloorSet{}, errors.New("release: invalid retained generation")
		}
	}
	pointer, err := readBoundedFloorFile(filepath.Join(s.path, "current"), 65)
	if errors.Is(err, os.ErrNotExist) {
		if len(entries) != 0 {
			return FloorSet{}, errors.New("release: retained history pointer missing")
		}
		return FloorSet{}, nil
	}
	if err != nil {
		return FloorSet{}, err
	}
	if len(pointer) != 65 || pointer[64] != '\n' || !floorGenerationName.Match(pointer[:64]) {
		return FloorSet{}, errors.New("release: invalid history pointer")
	}
	return readFloorGeneration(s.path, string(pointer[:64]))
}

func (s *floorStore) CommitRoot(v int64, d []byte, chain [][]byte) error {
	f, err := s.ReadFloors()
	if err != nil {
		return err
	}
	f.RootVersion = v
	f.RootDigest = append([]byte(nil), d...)
	return s.CommitFloors(f, chain)
}

func (s *floorStore) CommitFloors(next FloorSet, chain [][]byte) (resultErr error) {
	previous, err := s.ReadFloors()
	if err != nil {
		return err
	}
	if err = validateFloorSet(next); err != nil {
		return err
	}
	if err = assertFloorAdvance(previous, next); err != nil {
		return err
	}
	roots, err := validateRootArchive(chain, next)
	if err != nil {
		return err
	}
	payload, err := encodeFloorGeneration(next)
	if err != nil {
		return err
	}
	// After admission into physical publication, uncertainty retires this
	// owner. A later no-update cannot turn an observed unsynced pointer into
	// durable authorization merely by reading it from the filesystem cache.
	defer func() {
		if resultErr != nil {
			s.failure = resultErr
		}
	}()
	name := floorGenerationID(payload, roots)
	directory := filepath.Join(s.path, "generations", name)
	if _, err = os.Lstat(directory); errors.Is(err, os.ErrNotExist) {
		stage, err := os.MkdirTemp(filepath.Join(s.path, "generations"), ".stage-")
		if err != nil {
			return err
		}
		// Only this operation's freshly created direct staging directory may
		// be removed. Open never deletes arbitrary prefix-matching residue.
		published := false
		defer func() {
			if !published {
				resultErr = errors.Join(resultErr, os.RemoveAll(stage))
			}
		}()
		if err = writeSyncedFile(filepath.Join(stage, "state.bin"), payload); err != nil {
			return err
		}
		if err = writeRootArchive(stage, roots); err != nil {
			return err
		}
		if err = s.flush(stage); err != nil {
			return err
		}
		if err = durableRename(stage, directory); err != nil {
			return err
		}
		published = true
		if err = s.flush(filepath.Join(s.path, "generations")); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		stored, err := readFloorGeneration(s.path, name)
		if err != nil {
			return err
		}
		if !floorSetEqual(stored, next) {
			return errors.New("release: generation conflict")
		}
	}
	file, err := os.CreateTemp(s.path, ".current-")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer func() {
		err := os.Remove(temporary)
		if errors.Is(err, os.ErrNotExist) {
			err = nil
		}
		resultErr = errors.Join(resultErr, err)
	}()
	err = file.Chmod(0600)
	if err == nil {
		_, err = file.WriteString(name + "\n")
	}
	if err == nil {
		err = file.Sync()
	}
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}
	if err = durableRename(temporary, filepath.Join(s.path, "current")); err != nil {
		return err
	}
	return s.flush(s.path)
}

func (s *floorStore) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return errors.Join(s.failure, s.lease.release())
}

func floorGenerationID(payload []byte, roots []rootArchiveEntry) string {
	h := sha256.New()
	_, _ = h.Write(payload)
	for _, r := range roots {
		_, _ = h.Write(r.bytes)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func validateFloorSet(f FloorSet) error {
	if f.RootVersion <= 0 || len(f.RootDigest) != 32 {
		return errors.New("release: incomplete Root floor")
	}
	if f.TimestampVersion == 0 && len(f.TimestampDigest) == 0 && f.SnapshotVersion == 0 && len(f.SnapshotDigest) == 0 && f.TargetsVersion == 0 && len(f.TargetsDigest) == 0 {
		return nil
	}
	if f.TimestampVersion <= 0 || len(f.TimestampDigest) != 32 || f.SnapshotVersion <= 0 || len(f.SnapshotDigest) != 32 || f.TargetsVersion <= 0 || len(f.TargetsDigest) != 32 {
		return errors.New("release: incomplete metadata floors")
	}
	return nil
}
func assertFloorAdvance(old, next FloorSet) error {
	if old.RootVersion == 0 {
		return nil
	}
	before := []struct {
		v int64
		d []byte
	}{{old.RootVersion, old.RootDigest}, {old.TimestampVersion, old.TimestampDigest}, {old.SnapshotVersion, old.SnapshotDigest}, {old.TargetsVersion, old.TargetsDigest}}
	after := []struct {
		v int64
		d []byte
	}{{next.RootVersion, next.RootDigest}, {next.TimestampVersion, next.TimestampDigest}, {next.SnapshotVersion, next.SnapshotDigest}, {next.TargetsVersion, next.TargetsDigest}}
	for i, a := range after {
		b := before[i]
		if a.v < b.v || a.v == b.v && !bytes.Equal(a.d, b.d) {
			return fmt.Errorf("release: %s floor rollback or conflict", floorRoles[i])
		}
	}
	return nil
}
